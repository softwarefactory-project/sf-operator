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

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/cli/cmd/dev/gerrit"
	ms "github.com/softwarefactory-project/sf-operator/cli/cmd/dev/microshift"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/rest"

	apiroutev1 "github.com/openshift/api/route/v1"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var devCreateAllowedArgs = []string{"gerrit", "microshift", "standalone-sf", "demo-env"}
var devWipeAllowedArgs = []string{"gerrit", "sf"}
var devRunTestsAllowedArgs = []string{"olm", "standalone", "upgrade"}

var microshiftUser = "cloud-user"
var defaultDiskSpace = "20G"
var defaultHost = "microshift.dev"

var errMissingArg = errors.New("missing argument")

func ensureGatewayRoute(env *cliutils.ENV, fqdn string) {
	route := base.MkHTTPSRoute("sf-gateway", env.Ns, fqdn, "gateway", "/", 8080, map[string]string{})
	exists, _ := cliutils.GetM(env, "gateway", &apiroutev1.Route{})
	if !exists {
		cliutils.CreateROrDie(env, &route)
	}
}

func createDemoEnv(env cliutils.ENV, restConfig *rest.Config, fqdn string, reposPath, sfOperatorRepoPath string, keepDemoTenantDefinition bool) {
	ensureGatewayRoute(&env, fqdn)
	gerrit.EnsureGerrit(&env, fqdn)
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
	ctrl.Log.Info("Applying CRDs (did you run \"make manifests\" first?)...")
	ApplyCRDs(restConfig, sfOperatorRepoPath)
}

func createMicroshift(kmd *cobra.Command, cliCtx cliutils.SoftwareFactoryConfigContext) {
	skipLocalSetup, _ := kmd.Flags().GetBool("skip-local-setup")
	skipDeploy, _ := kmd.Flags().GetBool("skip-deploy")
	skipPostInstall, _ := kmd.Flags().GetBool("skip-post-install")
	dryRun, _ := kmd.Flags().GetBool("dry-run")
	rootDir := ms.CreateTempRootDir()

	msHost := cliCtx.Dev.Microshift.Host
	if msHost == "" {
		msHost = defaultHost
		ctrl.Log.Info("Host not set, defaulting to " + defaultHost)
	}
	msUser := cliCtx.Dev.Microshift.User
	if msUser == "" {
		// set default of "cloud-user" since these playbooks are meant to target a CentOS deployment
		msUser = microshiftUser
		ctrl.Log.Info("Host user not set, defaulting to " + microshiftUser)
	}
	msOpenshiftPullSecret := cliCtx.Dev.Microshift.OpenshiftPullSecret
	if msOpenshiftPullSecret == "" {
		msOpenshiftPullSecret = "not-a-valid-pull-secret"
		ctrl.Log.Info("A valid OpenShift pull secret must be set in `microshift` section of the configuration file")
	}
	msDiskFileSize := cliCtx.Dev.Microshift.DiskFileSize
	if msDiskFileSize == "" {
		msDiskFileSize = defaultDiskSpace
		ctrl.Log.Info("disk-file-size not set, defaulting to " + defaultDiskSpace)
	}
	msEtcdOnRamdisk := cliCtx.Dev.Microshift.ETCDOnRAMDisk
	msAnsibleMicroshiftRolePath := cliCtx.Dev.AnsibleMicroshiftRolePath
	if msAnsibleMicroshiftRolePath == "" {
		msAnsibleMicroshiftRolePath = rootDir + "/ansible-microshift-role"
		ctrl.Log.Info("No path to ansible-microshift-role provided, the role will be cloned into " + msAnsibleMicroshiftRolePath)
	}
	options := ms.MkAnsiblePlaybookOptions(msHost, msUser, msOpenshiftPullSecret, rootDir)
	varsFile := ms.MkTemporaryVarsFile(
		cliCtx.FQDN, msDiskFileSize, msAnsibleMicroshiftRolePath, rootDir, msEtcdOnRamdisk)
	options.ExtraVarsFile = []string{"@" + varsFile}
	// Ensure ansible-microshift-role is available
	ms.MkMicroshiftRoleSetupPlaybook(rootDir)
	if !dryRun {
		ms.RunMicroshiftRoleSetup(rootDir, cliCtx.Dev.SFOperatorRepositoryPath, msAnsibleMicroshiftRolePath, options)
	}
	// Ensure tooling and prerequisites are installed
	if !skipLocalSetup {
		ms.MkLocalSetupPlaybook(rootDir)
		if !dryRun {
			ms.RunLocalSetup(rootDir, cliCtx.Dev.SFOperatorRepositoryPath, msAnsibleMicroshiftRolePath, options)
		}
	}
	// Deploy MicroShift
	if !skipDeploy {
		ms.MkDeployMicroshiftPlaybook(rootDir)
		if !dryRun {
			ms.RunDeploy(rootDir, cliCtx.Dev.SFOperatorRepositoryPath, msAnsibleMicroshiftRolePath, options)
		}
	}
	// Configure cluster for development and testing
	if !skipPostInstall {
		ms.MkPostInstallPlaybook(rootDir)
		if !dryRun {
			ms.RunPostInstall(rootDir, cliCtx.Dev.SFOperatorRepositoryPath, msAnsibleMicroshiftRolePath, options)
		}
	}
	if !dryRun {
		defer os.RemoveAll(rootDir)
	} else {
		ctrl.Log.Info("Playbooks can be found in " + rootDir)
	}
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
		env := cliutils.ENV{
			Cli: cliutils.CreateKubernetesClientOrDie(kubeContext),
			Ctx: context.TODO(),
			Ns:  ns,
		}
		reposPath := cliCtx.Dev.Tests.DemoReposPath
		if reposPath == "" {
			ctrl.Log.Info("Demo repos path unset; repos will be cloned into ./deploy")
			reposPath = "deploy"
		}
		extraVars["demo_repos_path"] = reposPath
		createDemoEnv(env, restConfig, fqdn, reposPath, cliCtx.Dev.SFOperatorRepositoryPath, false)
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
	// we can't initialize an env if deploying microshift, so deal this case first and exit early
	if target == "microshift" {
		createMicroshift(kmd, cliCtx)
		return
	}
	env := cliutils.ENV{
		Cli: cliutils.CreateKubernetesClientOrDie(kubeContext),
		Ctx: context.TODO(),
		Ns:  ns,
	}
	if target == "gerrit" {
		gerrit.EnsureGerrit(&env, fqdn)
	} else if target == "standalone-sf" {
		sfResource, _ := kmd.Flags().GetString("cr")
		hasManifest := &cliCtx.Manifest
		if sfResource == "" && hasManifest != nil {
			sfResource = cliCtx.Manifest
		}
		if (sfResource != "" && ns == "") || (sfResource == "" && ns != "") {
			err := errors.New("standalone mode requires both --cr and --namespace to be set")
			ctrl.Log.Error(err, "Argument error:")
			os.Exit(1)
		}
		applyStandalone(ns, sfResource, kubeContext)
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
		createDemoEnv(env, restConfig, fqdn, reposPath, cliCtx.Dev.SFOperatorRepositoryPath, keepDemoTenantDefinition)

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
	var sf sfv1.SoftwareFactory
	sfDeleteOpts := []client.DeleteAllOfOption{
		client.InNamespace(ns),
	}
	if err := env.Cli.DeleteAllOf(env.Ctx, &sf, sfDeleteOpts...); err != nil {
		ctrl.Log.Info("SoftwareFactory resource not found")
	}
	var cm apiv1.ConfigMap
	cm.SetName("sf-standalone-owner")
	cm.SetNamespace(ns)
	if !cliutils.DeleteOrDie(env, &cm) {
		ctrl.Log.Info("standalone mode configmap not found")
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
	env := cliutils.ENV{
		Cli: cliutils.CreateKubernetesClientOrDie(kubeContext),
		Ctx: context.TODO(),
		Ns:  ns,
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
	env := cliutils.ENV{
		Cli: cliutils.CreateKubernetesClientOrDie(kubeContext),
		Ctx: context.TODO(),
		Ns:  ns,
	}
	gerrit.CloneAsAdmin(&env, fqdn, repoName, dest, verify)
}

func getImagesSecurityIssues(kmd *cobra.Command, args []string) {

	const quaySFBaseURL = "https://quay.io/api/v1/repository/software-factory/"

	type Vuln struct {
		Severity string
		Link     string
		Name     string
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

	getImageDigest := func(image base.Image) string {

		url := quaySFBaseURL + image.Name
		resp, _ := http.Get(url)
		target := Image{}
		json.NewDecoder(resp.Body).Decode(&target)

		return target.Tags[image.Version].ManifestDigest

	}

	getImageReport := func(image base.Image) {

		digest := getImageDigest(image)
		manifest := image.Name + "/manifest/" + digest
		url := quaySFBaseURL + manifest + "/security"
		resp, _ := http.Get(url)
		target := Scan{}
		json.NewDecoder(resp.Body).Decode(&target)

		println("\nScan result for: " + image.Name)
		for _, feature := range target.Data.Layer.Features {
			for _, vuln := range feature.Vulnerabilities {
				if vuln.Severity == "High" || vuln.Severity == "Critical" {
					fmt.Printf("- %s [%s] %s\n", feature.Name, vuln.Severity, vuln.Name)
				}
			}
		}
	}

	for _, image := range base.GetSelfManagedImages() {
		getImageReport(image)
	}

}

func MkDevCmd() *cobra.Command {

	var (
		deleteData              bool
		deleteOperator          bool
		verifyCloneSSL          bool
		msSkipDeploy            bool
		msSkipLocalSetup        bool
		msSkipPostInstall       bool
		msDryRun                bool
		sfResource              string
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
			Long:      "Create a development resource. The resource can be a MicroShift cluster, a standalone SF deployment, a demo environment or a gerrit instance",
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

	createCmd.Flags().BoolVar(&msSkipLocalSetup, "skip-local-setup", false, "(microshift) Do not install local requirements")
	createCmd.Flags().BoolVar(&msSkipDeploy, "skip-deploy", false, "(microshift) Do not deploy MicroShift")
	createCmd.Flags().BoolVar(&msSkipPostInstall, "skip-post-install", false, "(microshift) Do not setup namespace or install required operators")
	createCmd.Flags().BoolVar(&msDryRun, "dry-run", false, "(microshift) only create the playbook files, do not run them")

	createCmd.Flags().StringVar(&sfResource, "cr", "", "The path to the CR defining the Software Factory deployment.")

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
