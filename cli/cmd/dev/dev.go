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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/cli/cmd/dev/gerrit"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"

	apiroutev1 "github.com/openshift/api/route/v1"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var devCreateAllowedArgs = []string{"gerrit", "demo-env"}
var devWipeAllowedArgs = []string{"gerrit", "sf"}

func ensureGatewayRoute(env *controllers.SFKubeContext, fqdn string) {
	route := cliutils.MkHTTPSRoute("sf-gateway", env.Ns, fqdn, "gateway", "/", 8080, map[string]string{})
	exists := env.GetM("gateway", &apiroutev1.Route{})
	if !exists {
		env.CreateROrDie(&route)
	}
}

func createDemoEnv(env *controllers.SFKubeContext, fqdn string, reposPath, sfOperatorRepoPath string, keepDemoTenantDefinition bool, hostAliases []sfv1.HostAlias) {
	gerrit.EnsureGerrit(env, fqdn, hostAliases)
	if env.IsOpenShift {
		ensureGatewayRoute(env, fqdn)
		// TODO: write the gateway and gerrit ip to the local /etc/hosts, like we do for k8s
		// (this is presently done in the test suite)
	} else {
		cliutils.EnsureGatewayIngress(env, fqdn)
		cliutils.WriteIngressToEtcHosts(env, fqdn)
	}
	ctrl.Log.Info("Making sure Gerrit is up and ready...")
	gerrit.EnsureGerritAccess(fqdn)
	for _, repo := range []string{
		"config", "demo-tenant-config", "demo-project",
	} {
		ctrl.Log.Info("Cloning " + repo + "...")
		path := filepath.Join(reposPath, repo)
		gerrit.CloneAsAdmin(env, fqdn, repo, path, false)
	}
	SetupDemoConfigRepo(reposPath, "gerrit", "gerrit", !keepDemoTenantDefinition)
	SetupDemoProjectRepo(reposPath, fqdn)
	ctrl.Log.Info("Applying CRDs (did you run \"make manifests\" first?)")
	ApplyCRDs(env.RESTConfig, sfOperatorRepoPath)
}

func devCreate(kmd *cobra.Command, args []string) {
	env := cliutils.GetCLIContext(kmd)
	target := args[0]

	// The Gerrit container ip address is unknown and poting to 127.0.0.1
	// does not work as expected. In that case, point to the ingress
	// dpawlik: We are doing similar code in Ansible Microshift role
	// https://github.com/openstack-k8s-operators/ansible-microshift-role/blob/b48b04b96c1e819da28e535cc289ed25c81b2591/tasks/dnsmasq.yaml#L39
	hostAliases, _ := kmd.Flags().GetStringSlice("hostaliases")
	var hostAliasesSlice []sfv1.HostAlias
	for _, ha := range hostAliases {
		parts := strings.Split(ha, "=")
		hostAliasesSlice = append(hostAliasesSlice, sfv1.HostAlias{IP: parts[0], Hostnames: strings.Split(parts[1], ",")})
	}

	fqdn, _ := kmd.Flags().GetString("fqdn")
	if fqdn == "" {
		fqdn = "sfop.me"
	}

	if target == "gerrit" {
		gerrit.EnsureGerrit(env, fqdn, hostAliasesSlice)
	} else if target == "demo-env" {
		reposPath, _ := kmd.Flags().GetString("repos-path")
		if reposPath == "" {
			reposPath, _ = kmd.Flags().GetString("demo-repos-path")
		}
		if reposPath == "" {
			ctrl.Log.Info("Demo repos path unset; repos will be cloned into ./deploy")
			reposPath = "deploy"
		}
		keepDemoTenantDefinition, _ := kmd.Flags().GetBool("keep-demo-tenant")
		sfOperatorRepoPath, _ := kmd.Flags().GetString("sf-operator-repository-path")
		createDemoEnv(env, fqdn, reposPath, sfOperatorRepoPath, keepDemoTenantDefinition, hostAliasesSlice)

	} else {
		ctrl.Log.Error(errors.New("unsupported target"), "Invalid argument '"+target+"'")
	}
}

func devWipe(kmd *cobra.Command, args []string) {
	env := cliutils.GetCLIContext(kmd)
	target := args[0]
	rmData, _ := kmd.Flags().GetBool("rm-data")
	if target == "gerrit" {
		gerrit.WipeGerrit(env, rmData)
	} else if target == "sf" {
		env.CleanSFInstance()
		if rmData {
			ctrl.Log.Info("Removing dangling persistent volume claims if any...")
			env.CleanPVCs()
		}
	} else {
		ctrl.Log.Error(errors.New("unsupported target"), "Invalid argument '"+target+"'")
	}
}

func devCloneAsAdmin(kmd *cobra.Command, args []string) {
	env := cliutils.GetCLIContext(kmd)
	repoName := args[0]
	var dest string
	if len(args) > 1 {
		dest = args[1]
	} else {
		dest = "."
	}
	fqdn, _ := kmd.Flags().GetString("fqdn")
	if fqdn == "" {
		fqdn = "sfop.me"
	}
	verify, _ := kmd.Flags().GetBool("verify")
	gerrit.CloneAsAdmin(env, fqdn, repoName, dest, verify)
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
		verifyCloneSSL          bool
		msSkipPostInstall       bool
		msDryRun                bool
		demoEnvReposPath        string
		demoEnvKeepTenantConfig bool
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
		getImagesSecurityIssuesCmd = &cobra.Command{
			Use:  "getImagesSecurityIssues",
			Long: "Return the list of security issues reported by Quay.io (only High and Critical)",
			Run:  getImagesSecurityIssues,
		}
	)
	// args
	wipeCmd.Flags().BoolVar(&deleteData, "rm-data", false, "Delete also persistent data")

	cloneAsAdminCmd.Flags().BoolVar(&verifyCloneSSL, "verify", false, "Verify SSL endpoint")

	createCmd.Flags().BoolVar(&msSkipPostInstall, "skip-post-install", false, "(microshift) Do not setup namespace or install required operators")
	createCmd.Flags().BoolVar(&msDryRun, "dry-run", false, "(microshift) only create the playbook files, do not run them")

	createCmd.Flags().BoolVar(&demoEnvKeepTenantConfig, "keep-tenant-config", false, "(demo-env) Do not update the demo tenant configuration")
	createCmd.Flags().StringVar(&demoEnvReposPath, "repos-path", "", "(demo-env) the path to clone demo repos at")

	devCmd.AddCommand(createCmd)
	devCmd.AddCommand(wipeCmd)
	devCmd.AddCommand(cloneAsAdminCmd)

	devCmd.AddCommand(getImagesSecurityIssuesCmd)

	return devCmd
}
