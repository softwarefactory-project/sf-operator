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
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/softwarefactory-project/sf-operator/cli/cmd/dev/gerrit"
	ms "github.com/softwarefactory-project/sf-operator/cli/cmd/dev/microshift"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/controllers"
	"k8s.io/client-go/rest"

	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var devCreateAllowedArgs = []string{"gerrit", "microshift", "standalone-sf", "demo-env"}
var devWipeAllowedArgs = []string{"gerrit"}
var devRunTestsAllowedArgs = []string{"olm", "standalone", "upgrade"}

var microshiftUser = "cloud-user"
var defaultDiskSpace = "20G"

var errMissingArg = errors.New("missing argument")

func createDemoEnv(env cliutils.ENV, restConfig *rest.Config, fqdn string, reposPath, sfOperatorRepoPath string, keepDemoTenantDefinition bool) {

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
		ctrl.Log.Error(errMissingArg, "Host must be set in `microshift` section of the configuration")
		os.Exit(1)
	}
	msUser := cliCtx.Dev.Microshift.User
	if msUser == "" {
		// set default of "cloud-user" since these playbooks are meant to target a CentOS deployment
		msUser = microshiftUser
		ctrl.Log.Info("Host user not set, defaulting to " + microshiftUser)
	}
	msOpenshiftPullSecret := cliCtx.Dev.Microshift.OpenshiftPullSecret
	if msOpenshiftPullSecret == "" {
		ctrl.Log.Error(errMissingArg, "A valid OpenShift pull secret must be set in `microshift` section of the configuration")
		os.Exit(1)
	}
	msDiskFileSize := cliCtx.Dev.Microshift.DiskFileSize
	if msDiskFileSize == "" {
		msDiskFileSize = defaultDiskSpace
		ctrl.Log.Info("disk-file-size not set, defaulting to " + defaultDiskSpace)
	}
	msAnsibleMicroshiftRolePath := cliCtx.Dev.AnsibleMicroshiftRolePath
	if msAnsibleMicroshiftRolePath == "" {
		msAnsibleMicroshiftRolePath = rootDir + "/ansible-microshift-role"
		ctrl.Log.Info("No path to ansible-microshift-role provided, the role will be cloned into " + msAnsibleMicroshiftRolePath)
	}
	msSFOperatorRepositoryPath := cliCtx.Dev.SFOperatorRepositoryPath
	if msSFOperatorRepositoryPath == "" {
		ctrl.Log.Error(errMissingArg, "The path to the sf-operator repository must be set in `dev` section of the configuration")
		os.Exit(1)
	}

	options := ms.MkAnsiblePlaybookOptions(msHost, msUser, msOpenshiftPullSecret, rootDir)
	varsFile := ms.MkTemporaryVarsFile(cliCtx.FQDN, msDiskFileSize, msAnsibleMicroshiftRolePath, rootDir)
	options.ExtraVarsFile = []string{"@" + varsFile}
	// Ensure ansible-microshift-role is available
	ms.MkMicroshiftRoleSetupPlaybook(rootDir)
	if !dryRun {
		ms.RunMicroshiftRoleSetup(rootDir, msSFOperatorRepositoryPath, msAnsibleMicroshiftRolePath, options)
	}
	// Ensure tooling and prerequisites are installed
	if !skipLocalSetup {
		ms.MkLocalSetupPlaybook(rootDir)
		if !dryRun {
			ms.RunLocalSetup(rootDir, msSFOperatorRepositoryPath, msAnsibleMicroshiftRolePath, options)
		}
	}
	// Deploy MicroShift
	if !skipDeploy {
		ms.MkDeployMicroshiftPlaybook(rootDir)
		if !dryRun {
			ms.RunDeploy(rootDir, msSFOperatorRepositoryPath, msAnsibleMicroshiftRolePath, options)
		}
	}
	// Configure cluster for development and testing
	if !skipPostInstall {
		ms.MkPostInstallPlaybook(rootDir)
		if !dryRun {
			ms.RunPostInstall(rootDir, msSFOperatorRepositoryPath, msAnsibleMicroshiftRolePath, options)
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
	sfOperatorRepositoryPath := cliCtx.Dev.SFOperatorRepositoryPath
	vars, _ := kmd.Flags().GetStringSlice("extra-var")
	extraVars := cliutils.VarListToMap(vars)
	if len(extraVars) == 0 {
		extraVars = cliCtx.Dev.Tests.ExtraVars
	}
	if sfOperatorRepositoryPath == "" {
		ctrl.Log.Error(errMissingArg, "The path to the sf-operator repository must be set in `dev` section of the configuration")
		os.Exit(1)
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
		// overwrite demo_repos_path ansible variable
		if extraVars == nil {
			extraVars = make(map[string]string)
		}
		extraVars["demo_repos_path"] = reposPath
		createDemoEnv(env, restConfig, fqdn, reposPath, sfOperatorRepositoryPath, false)
	}
	if target == "olm" {
		runTestOLM(extraVars, sfOperatorRepositoryPath, verbosity)
	} else if target == "standalone" {
		runTestStandalone(extraVars, sfOperatorRepositoryPath, verbosity)
	} else if target == "upgrade" {
		runTestUpgrade(extraVars, sfOperatorRepositoryPath, verbosity)
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
		sfOperatorRepositoryPath := cliCtx.Dev.SFOperatorRepositoryPath
		if sfOperatorRepositoryPath == "" {
			ctrl.Log.Error(errMissingArg, "The path to the sf-operator repository must be set in `dev` section of the configuration")
			os.Exit(1)
		}
		keepDemoTenantDefinition, _ := kmd.Flags().GetBool("keep-demo-tenant")
		createDemoEnv(env, restConfig, fqdn, reposPath, sfOperatorRepositoryPath, keepDemoTenantDefinition)

	} else {
		ctrl.Log.Error(errors.New("unsupported target"), "Invalid argument '"+target+"'")
	}
}

func devWipe(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIctxOrDie(kmd, args, devCreateAllowedArgs)
	target := args[0]
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	if target == "gerrit" {
		rmData, _ := kmd.Flags().GetBool("rm-data")
		env := cliutils.ENV{
			Cli: cliutils.CreateKubernetesClientOrDie(kubeContext),
			Ctx: context.TODO(),
			Ns:  ns,
		}
		gerrit.WipeGerrit(&env, rmData)
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

func MkDevCmd() *cobra.Command {

	var (
		deleteData              bool
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
	)
	// args
	wipeCmd.Flags().BoolVar(&deleteData, "rm-data", false, "Delete also persistent data. This will result in data loss, like review history.")

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
	return devCmd
}
