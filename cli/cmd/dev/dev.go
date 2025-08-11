/*
Copyright © 2023 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package dev subcommands can be used to manage a development environment and run tests.
package dev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/cli/cmd/dev/gerrit"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/rest"

	apiroutev1 "github.com/openshift/api/route/v1"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var devCreateAllowedArgs = []string{"gerrit", "demo-env"}
var devWipeAllowedArgs = []string{"gerrit", "sf"}
var devRunTestsAllowedArgs = []string{"olm", "standalone", "upgrade"}

var errMissingArg = errors.New("missing argument")

func ensureGatewayRoute(env *cliutils.ENV, fqdn string) {
	route := cliutils.MkHTTPSRoute("sf-gateway", env.Ns, fqdn, "gateway", "/", 8080, map[string]string{})
	exists, _ := cliutils.GetM(env, "gateway", &apiroutev1.Route{})
	if !exists {
		cliutils.CreateROrDie(env, &route)
	}
}

func createDemoEnv(env cliutils.ENV, restConfig *rest.Config, fqdn string, reposPath, sfOperatorRepoPath string, keepDemoTenantDefinition bool, hostAliases []sfv1.HostAlias) {
	gerrit.EnsureGerrit(&env, fqdn, hostAliases)
	if env.IsOpenShift {
		ensureGatewayRoute(&env, fqdn)
		// TODO: write the gateway and gerrit ip to the local /etc/hosts, like we do for k8s
		// (this is presently done in the test suite)
	} else {
		cliutils.EnsureGatewayIngress(&env, fqdn)
		cliutils.WriteIngressToEtcHosts(&env, fqdn)
	}
	ctrl.Log.Info("Making sure Gerrit is up and ready...")
	gerrit.EnsureGerritAccess(fqdn)
	for _, repo := range []string{
		"config", "demo-tenant-config", "demo-project",
	} {
		ctrl.Log.Info("Cloning " + repo + "...")
		path := filepath.Join(reposPath, repo)
		gerrit.CloneAsAdmin(&env, fqdn, repo, path, false)
	}
	SetupDemoConfigRepo(reposPath, "gerrit", "gerrit", !keepDemoTenantDefinition)
	SetupDemoProjectRepo(reposPath)
	ctrl.Log.Info("Applying CRDs (did you run \"make manifests\" first?)")
	ApplyCRDs(restConfig, sfOperatorRepoPath)
}

func devRunTests(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIctxOrDie(kmd, args, runTestsAllowedArgs)
	target := args[0]
	vars, _ := kmd.Flags().GetStringSlice("extra-var")
	extraVars := cliutils.VarListToMap(vars)
	if len(extraVars) == 0 {
		extraVars = cliCtx.Dev.Tests.ExtraVars
	}
	if extraVars == nil {
		extraVars = make(map[string]string)
	}
	var verbosity string
	verbose, _ := kmd.Flags().GetBool("v")
	debug, _ := kmd.Flags().GetBool("vvv")
	prepareDemoEnv, _ := kmd.Flags().GetBool("prepare-demo-env")
	if verbose {
		verbosity = "verbose"
	}
	if debug {
		verbosity = "debug"
	}

	if prepareDemoEnv {
		ns := cliCtx.Namespace
		kubeContext := cliCtx.KubeContext
		restConfig := controllers.GetConfigContextOrDie(kubeContext)
		fqdn := cliCtx.FQDN
		client := cliutils.CreateKubernetesClientOrDie(kubeContext)
		ctx := context.TODO()
		env := cliutils.ENV{
			Cli:         client,
			Ctx:         ctx,
			Ns:          ns,
			IsOpenShift: controllers.CheckOpenShift(),
		}
		reposPath := cliCtx.Dev.Tests.DemoReposPath
		if reposPath == "" {
			ctrl.Log.Info("Demo repos path unset; repos will be cloned into ./deploy")
			reposPath = "deploy"
		}
		extraVars["demo_repos_path"] = reposPath

		// The Gerrit container ip address is unknown and poting to 127.0.0.1
		// does not work as expected. In that case, point to the ingress
		// dpawlik: We are doing similar code in Ansible Microshift role
		// https://github.com/openstack-k8s-operators/ansible-microshift-role/blob/b48b04b96c1e819da28e535cc289ed25c81b2591/tasks/dnsmasq.yaml#L39
		hostAliases := cliCtx.HostAliases
		createDemoEnv(env, restConfig, fqdn, reposPath, cliCtx.Dev.SFOperatorRepositoryPath, false, hostAliases)
	}
	// use config file and context for CLI calls in the tests
	var cliGlobalFlags string
	configPath, _ := kmd.Flags().GetString("config")
	cliContext, _ := kmd.Flags().GetString("context")
	if configPath == "" {
		ctrl.Log.Error(errMissingArg, "A CLI configuration file with a development/testing context is required")
		os.Exit(1)
	}
	cliGlobalFlags = "--config " + configPath + " "
	if cliContext != "" {
		cliGlobalFlags += "--context " + cliContext + " "
	}
	extraVars["cli_global_flags"] = cliGlobalFlags
	if target == "olm" {
		runTestOLM(extraVars, cliCtx.Dev.SFOperatorRepositoryPath, verbosity)
	} else if target == "standalone" {
		runTestStandalone(extraVars, cliCtx.Dev.SFOperatorRepositoryPath, verbosity)
	} else if target == "upgrade" {
		runTestUpgrade(extraVars, cliCtx.Dev.SFOperatorRepositoryPath, verbosity)
	}
}

func devCreate(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIctxOrDie(kmd, args, devCreateAllowedArgs)
	target := args[0]
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	fqdn := cliCtx.FQDN
	client := cliutils.CreateKubernetesClientOrDie(kubeContext)
	ctx := context.TODO()

	env := cliutils.ENV{
		Cli:         client,
		Ctx:         ctx,
		Ns:          ns,
		IsOpenShift: controllers.CheckOpenShift(),
	}

	// The Gerrit container ip address is unknown and poting to 127.0.0.1
	// does not work as expected. In that case, point to the ingress
	// dpawlik: We are doing similar code in Ansible Microshift role
	// https://github.com/openstack-k8s-operators/ansible-microshift-role/blob/b48b04b96c1e819da28e535cc289ed25c81b2591/tasks/dnsmasq.yaml#L39
	hostAliases := cliCtx.HostAliases

	if target == "gerrit" {
		gerrit.EnsureGerrit(&env, fqdn, hostAliases)
	} else if target == "demo-env" {
		restConfig := controllers.GetConfigContextOrDie(kubeContext)
		reposPath, _ := kmd.Flags().GetString("repos-path")
		if reposPath == "" {
			reposPath = cliCtx.Dev.Tests.DemoReposPath
		}
		if reposPath == "" {
			ctrl.Log.Info("Demo repos path unset; repos will be cloned into ./deploy")
			reposPath = "deploy"
		}
		keepDemoTenantDefinition, _ := kmd.Flags().GetBool("keep-demo-tenant")

		createDemoEnv(env, restConfig, fqdn, reposPath, cliCtx.Dev.SFOperatorRepositoryPath, keepDemoTenantDefinition, hostAliases)

	} else {
		ctrl.Log.Error(errors.New("unsupported target"), "Invalid argument '"+target+"'")
	}
}

func getOperatorSelector() labels.Selector {
	selector := labels.NewSelector()
	req, err := labels.NewRequirement(
		"operators.coreos.com/sf-operator.operators",
		selection.Exists,
		[]string{})
	if err != nil {
		ctrl.Log.Error(err, "could not set label selector to clean subscriptions")
		os.Exit(1)
	}
	return selector.Add(*req)
}

func cleanSubscription(env *cliutils.ENV) {
	selector := getOperatorSelector()

	subscriptionListOpts := []client.ListOption{
		client.InNamespace("operators"),
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	}

	subsList := v1alpha1.SubscriptionList{}
	if err := env.Cli.List(env.Ctx, &subsList, subscriptionListOpts...); err != nil {
		ctrl.Log.Error(err, "error listing subscriptions")
		os.Exit(1)
	}
	if len(subsList.Items) > 0 {
		subscriptionDeleteOpts := []client.DeleteAllOfOption{
			client.InNamespace("operators"),
			client.MatchingLabelsSelector{
				Selector: selector,
			},
		}
		sub := v1alpha1.Subscription{}
		cliutils.DeleteAllOfOrDie(env, &sub, subscriptionDeleteOpts...)
	}
}

func cleanCatalogSource(env *cliutils.ENV) {
	cs := v1alpha1.CatalogSource{}
	cs.SetName("sf-operator-catalog")
	cs.SetNamespace("operators")
	if !cliutils.DeleteOrDie(env, &cs) {
		ctrl.Log.Info("CatalogSource \"sf-operator-catalog\" not found")
	}
}

func cleanClusterServiceVersion(env *cliutils.ENV) {
	selector := getOperatorSelector()

	subscriptionListOpts := []client.ListOption{
		client.InNamespace("operators"),
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	}

	csvsList := v1alpha1.ClusterServiceVersionList{}
	if err := env.Cli.List(env.Ctx, &csvsList, subscriptionListOpts...); err != nil {
		ctrl.Log.Error(err, "error listing cluster service versions")
		os.Exit(1)
	}
	if len(csvsList.Items) > 0 {
		csvDeleteOpts := []client.DeleteAllOfOption{
			client.InNamespace("operators"),
			client.MatchingLabelsSelector{
				Selector: selector,
			},
		}
		csv := v1alpha1.ClusterServiceVersion{}
		cliutils.DeleteAllOfOrDie(env, &csv, csvDeleteOpts...)
	}
}

func cleanSFInstance(env *cliutils.ENV, ns string) {
	// --- Scale down Zuul STS components
	components := []string{"zuul-executor", "zuul-merger", "zuul-scheduler"}
	ctrl.Log.Info("Scaling down StatefulSets before deletion...")
	for _, comp := range components {
		if err := cliutils.ScaleDownSTSAndWait(env, ns, comp); err != nil {
			ctrl.Log.Error(err, "Could not scale down StatefulSet, continuing...", "name", comp)
		}
	}

	// --- Detection and Deletion Logic ---
	var sfList sfv1.SoftwareFactoryList
	if err := env.Cli.List(env.Ctx, &sfList, client.InNamespace(ns)); err != nil {
		ctrl.Log.Error(err, "API error when checking for SoftwareFactory resource")
	}

	var cm apiv1.ConfigMap
	cmKey := client.ObjectKey{Namespace: ns, Name: "sf-standalone-owner"}
	cmErr := env.Cli.Get(env.Ctx, cmKey, &cm)

	const maxRetries = 60
	const retryInterval = 2 * time.Second

	if len(sfList.Items) > 0 {
		// --- Operator Mode Deletion ---
		ctrl.Log.Info("Operator mode detected. Deleting SoftwareFactory resource(s) with foreground propagation.")
		var sf sfv1.SoftwareFactory
		sfDeleteOpts := []client.DeleteAllOfOption{
			client.InNamespace(ns),
			client.PropagationPolicy(metav1.DeletePropagationForeground),
		}
		if err := env.Cli.DeleteAllOf(env.Ctx, &sf, sfDeleteOpts...); err != nil {
			ctrl.Log.Error(err, "Failed to initiate deletion of SoftwareFactory resources")
			return
		}

		// Wait for SoftwareFactory resources to be fully deleted
		ctrl.Log.Info("Waiting for SoftwareFactory resources to be fully deleted...")
		deleted := false
		for i := 0; i < maxRetries; i++ {
			var currentSfList sfv1.SoftwareFactoryList
			env.Cli.List(env.Ctx, &currentSfList, client.InNamespace(ns))
			if len(currentSfList.Items) == 0 {
				ctrl.Log.Info("SoftwareFactory resources successfully deleted.")
				deleted = true
				break
			}
			time.Sleep(retryInterval)
		}
		if !deleted {
			ctrl.Log.Info("Timed out waiting for SoftwareFactory resources to be deleted.")
		}

	} else if cmErr == nil {
		// --- Standalone Mode Deletion ---
		ctrl.Log.Info("Standalone mode detected. Deleting owner ConfigMap with foreground propagation.")
		deleteOpts := &client.DeleteOptions{
			PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
		}
		if err := env.Cli.Delete(env.Ctx, &cm, deleteOpts); err != nil {
			ctrl.Log.Error(err, "Failed to initiate deletion of standalone ConfigMap")
			return
		}

		// Wait for the ConfigMap to be fully deleted
		ctrl.Log.Info("Waiting for the owner ConfigMap to be fully deleted...")
		deleted := false
		for i := 0; i < maxRetries; i++ {
			var checkCm apiv1.ConfigMap
			err := env.Cli.Get(env.Ctx, cmKey, &checkCm)
			if apierrors.IsNotFound(err) {
				ctrl.Log.Info("Standalone owner ConfigMap successfully deleted.")
				deleted = true
				break
			}
			time.Sleep(retryInterval)
		}
		if !deleted {
			ctrl.Log.Info("Timed out waiting for the owner ConfigMap to be deleted.")
		}

	} else {
		ctrl.Log.Info("No SoftwareFactory resource or standalone ConfigMap found. Instance is already clean.")
	}
}

func cleanPVCs(env *cliutils.ENV, ns string) {
	selector := labels.NewSelector()
	appReq, err := labels.NewRequirement(
		"app",
		selection.In,
		[]string{"sf"})
	if err != nil {
		ctrl.Log.Error(err, "could not set app label requirement to clean PVCs")
		os.Exit(1)
	}
	runReq, err := labels.NewRequirement(
		"run",
		selection.NotIn,
		[]string{"gerrit"})
	if err != nil {
		ctrl.Log.Error(err, "could not set run label requirement to clean PVCs")
		os.Exit(1)
	}
	selector = selector.Add([]labels.Requirement{*appReq, *runReq}...)
	pvcDeleteOpts := []client.DeleteAllOfOption{
		client.InNamespace(ns),
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	}
	var pvc apiv1.PersistentVolumeClaim
	cliutils.DeleteAllOfOrDie(env, &pvc, pvcDeleteOpts...)
}

func devWipe(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIctxOrDie(kmd, args, devWipeAllowedArgs)
	target := args[0]
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	rmData, _ := kmd.Flags().GetBool("rm-data")
	rmOp, _ := kmd.Flags().GetBool("rm-operator")
	client := cliutils.CreateKubernetesClientOrDie(kubeContext)
	ctx := context.TODO()
	env := cliutils.ENV{
		Cli:         client,
		Ctx:         ctx,
		Ns:          ns,
		IsOpenShift: controllers.CheckOpenShift(),
	}
	if target == "gerrit" {
		gerrit.WipeGerrit(&env, rmData)
	} else if target == "sf" {
		cleanSFInstance(&env, ns)
		if rmData {
			ctrl.Log.Info("Removing dangling persistent volume claims if any...")
			cleanPVCs(&env, ns)
		}
		if rmOp {
			ctrl.Log.Info("Removing SF Operator if present...")
			cleanSubscription(&env)
			cleanCatalogSource(&env)
			cleanClusterServiceVersion(&env)
		}
	} else {
		ctrl.Log.Error(errors.New("unsupported target"), "Invalid argument '"+target+"'")
	}
}

func devCloneAsAdmin(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIctxOrDie(kmd, args, []string{})
	repoName := args[0]
	var dest string
	if len(args) > 1 {
		dest = args[1]
	} else {
		dest = "."
	}
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	fqdn := cliCtx.FQDN
	verify, _ := kmd.Flags().GetBool("verify")
	client := cliutils.CreateKubernetesClientOrDie(kubeContext)
	ctx := context.TODO()
	env := cliutils.ENV{
		Cli:         client,
		Ctx:         ctx,
		Ns:          ns,
		IsOpenShift: controllers.CheckOpenShift(),
	}
	gerrit.CloneAsAdmin(&env, fqdn, repoName, dest, verify)
}

func getImagesSecurityIssues(kmd *cobra.Command, args []string) {

	const quayBaseURL = "https://quay.io/api/v1/repository/"

	// Quay.io data struct
	type Vuln struct {
		Severity string
		Link     string
		Name     string
		FixedBy  string
	}

	type Feature struct {
		Name            string
		Vulnerabilities []Vuln
	}

	type Layer struct {
		Features []Feature
	}

	type Data struct {
		Layer Layer
	}

	type Scan struct {
		Status string
		Data   Data
	}

	type Tag struct {
		ManifestDigest string `json:"manifest_digest"`
	}

	type Image struct {
		Name string
		Tags map[string]Tag
	}
	// -- end -- Quay.io data struct

	// Final data struct
	type ImageAdvisories struct {
		image         string
		hash          string
		url           string
		highCount     int
		criticalCount int
		fixableCount  int
		advisories    []Vuln
	}

	type SFOPImagesAdvisories struct {
		imagesAdvisories []ImageAdvisories
		highCount        int
		criticalCount    int
		fixableCount     int
	}
	// - end Final data struct

	getContainerPath := func(image base.Image) string {
		index := strings.Index(image.Container, "/")
		if index == -1 {
			println("Unable to extract the container path", image.Container)
			os.Exit(1)
		}
		return image.Container[index+1:]
	}

	getImageDigest := func(image base.Image) string {
		container := getContainerPath(image)
		url := quayBaseURL + container
		resp, _ := http.Get(url)
		target := Image{}
		json.NewDecoder(resp.Body).Decode(&target)

		tag, exists := target.Tags[image.Version]
		if !exists {
			println("Unable to find the image by name on software-factory organization on quay.io")
			os.Exit(1)
		}
		return tag.ManifestDigest
	}

	getImageAdvisories := func(image base.Image) ImageAdvisories {
		digest := getImageDigest(image)
		container := getContainerPath(image)
		manifest := container + "/manifest/" + digest
		url := quayBaseURL + manifest + "/security"
		resp, _ := http.Get(url)
		target := Scan{}
		json.NewDecoder(resp.Body).Decode(&target)

		println("Fetching scanning result for: " + image.Container)
		advs := ImageAdvisories{
			image:         image.Container,
			hash:          digest,
			highCount:     0,
			criticalCount: 0,
			fixableCount:  0,
			url:           fmt.Sprintf("https://quay.io/repository/%s/manifest/%s?tab=vulnerabilities", container, digest),
			advisories:    []Vuln{},
		}
		for _, feature := range target.Data.Layer.Features {
			for _, vuln := range feature.Vulnerabilities {
				if vuln.Severity == "High" || vuln.Severity == "Critical" {
					advs.advisories = append(advs.advisories, vuln)
				}
			}
		}
		for _, vuln := range advs.advisories {
			if vuln.Severity == "High" {
				advs.highCount += 1
				if len(vuln.FixedBy) > 0 {
					advs.fixableCount += 1
				}
			}
			if vuln.Severity == "Critical" {
				advs.criticalCount += 1
				if len(vuln.FixedBy) > 0 {
					advs.fixableCount += 1
				}
			}
		}
		return advs
	}

	displayAdvisories := func(advs SFOPImagesAdvisories) {
		for _, imageAdvisories := range advs.imagesAdvisories {
			fmt.Printf(
				"\nContainer Image: %s (Criticals: %d, Highs: %d), (Fixables: %d)\n",
				imageAdvisories.image, imageAdvisories.criticalCount,
				imageAdvisories.highCount, imageAdvisories.fixableCount)
			println(imageAdvisories.url)
		}
	}

	writePromAdvisories := func(advs SFOPImagesAdvisories) {
		filepath, _ := utils.GetEnvVarValue("PROM_TEXT_FILE")
		if filepath != "" {
			output := ""
			for _, imageAdvisories := range advs.imagesAdvisories {
				output += fmt.Sprintf(
					"sf_operator_image_advisories{image=\"%s\",severity=\"high\"} %d\n",
					imageAdvisories.image, imageAdvisories.highCount)
				output += fmt.Sprintf(
					"sf_operator_image_advisories{image=\"%s\",severity=\"critical\"} %d\n",
					imageAdvisories.image, imageAdvisories.criticalCount)
				output += fmt.Sprintf(
					"sf_operator_image_advisories_fixables{image=\"%s\"} %d\n",
					imageAdvisories.image, imageAdvisories.fixableCount)
			}
			output += fmt.Sprintf(
				"sf_operator_advisories{severity=\"high\"} %d\n", advs.highCount)
			output += fmt.Sprintf(
				"sf_operator_advisories{severity=\"critical\"} %d\n", advs.criticalCount)
			output += fmt.Sprintf(
				"sf_operator_advisories_fixable{} %d\n", advs.fixableCount)
			os.WriteFile(filepath, []byte(output), 0644)
			println()
			println(output)
		}
	}

	sfopAdvisories := SFOPImagesAdvisories{
		imagesAdvisories: []ImageAdvisories{},
		highCount:        0,
		criticalCount:    0,
		fixableCount:     0,
	}

	for _, image := range base.GetQuayImages() {
		imageAdvisories := getImageAdvisories(image)
		sfopAdvisories.imagesAdvisories = append(sfopAdvisories.imagesAdvisories, imageAdvisories)
		sfopAdvisories.highCount += imageAdvisories.highCount
		sfopAdvisories.criticalCount += imageAdvisories.criticalCount
		sfopAdvisories.fixableCount += imageAdvisories.fixableCount
	}

	displayAdvisories(sfopAdvisories)
	writePromAdvisories(sfopAdvisories)

}

func MkDevCmd() *cobra.Command {

	var (
		deleteData              bool
		deleteOperator          bool
		verifyCloneSSL          bool
		msSkipPostInstall       bool
		msDryRun                bool
		extraVars               []string
		testVerbose             bool
		testDebug               bool
		demoEnvReposPath        string
		demoEnvKeepTenantConfig bool
		prepareDemoEnv          bool
		devCmd                  = &cobra.Command{
			Use:   "dev",
			Short: "development subcommands",
			Long:  "These subcommands can be used to manage a dev environment and streamline recurrent development tasks like running the operator's test suite.",
		}
		createCmd = &cobra.Command{
			Use:       "create {" + strings.Join(devCreateAllowedArgs, ", ") + "}",
			Long:      "Create a development resource. The resource can be a MicroShift cluster, a demo environment or a gerrit instance",
			ValidArgs: devCreateAllowedArgs,
			Run:       devCreate,
		}
		wipeCmd = &cobra.Command{
			Use:       "wipe {" + strings.Join(devWipeAllowedArgs, ", ") + "}",
			Long:      "Wipe a development resource. The resource can be a gerrit instance.",
			ValidArgs: devWipeAllowedArgs,
			Run:       devWipe,
		}
		cloneAsAdminCmd = &cobra.Command{
			Use:  "cloneAsAdmin REPO [DEST]",
			Long: "Clone a repo hosted on the dev code review system as an admin user.",
			Run:  devCloneAsAdmin,
		}
		runTestsCmd = &cobra.Command{
			Use:       "run-tests TESTNAME",
			Long:      "Runs a test suite locally. TESTNAME can be `olm`, `standalone` or `upgrade`. A demo environment must be ready before running the tests, either by invoking `dev create demo-env` or using the `--prepare-demo-env` flag",
			ValidArgs: devRunTestsAllowedArgs,
			Run:       devRunTests,
		}
		getImagesSecurityIssuesCmd = &cobra.Command{
			Use:  "getImagesSecurityIssues",
			Long: "Return the list of security issues reported by Quay.io (only High and Critical)",
			Run:  getImagesSecurityIssues,
		}
	)
	// args
	wipeCmd.Flags().BoolVar(&deleteData, "rm-data", false, "Delete also persistent data")
	wipeCmd.Flags().BoolVar(&deleteOperator, "rm-operator", false, "[sf] Delete also the operator installation")

	cloneAsAdminCmd.Flags().BoolVar(&verifyCloneSSL, "verify", false, "Verify SSL endpoint")

	createCmd.Flags().BoolVar(&msSkipPostInstall, "skip-post-install", false, "(microshift) Do not setup namespace or install required operators")
	createCmd.Flags().BoolVar(&msDryRun, "dry-run", false, "(microshift) only create the playbook files, do not run them")

	createCmd.Flags().BoolVar(&demoEnvKeepTenantConfig, "keep-tenant-config", false, "(demo-env) Do not update the demo tenant configuration")
	createCmd.Flags().StringVar(&demoEnvReposPath, "repos-path", "", "(demo-env) the path to clone demo repos at")

	runTestsCmd.Flags().StringSliceVar(&extraVars, "extra-var", []string{}, "Set an extra variable in the form `key=value` to pass to the test playbook. Repeatable")
	runTestsCmd.Flags().BoolVar(&testVerbose, "v", false, "run ansible in verbose mode")
	runTestsCmd.Flags().BoolVar(&testDebug, "vvv", false, "run ansible in debug mode")
	runTestsCmd.Flags().BoolVar(&prepareDemoEnv, "prepare-demo-env", false, "prepare demo environment")

	devCmd.AddCommand(createCmd)
	devCmd.AddCommand(wipeCmd)
	devCmd.AddCommand(cloneAsAdminCmd)
	devCmd.AddCommand(runTestsCmd)

	devCmd.AddCommand(getImagesSecurityIssuesCmd)

	return devCmd
}
