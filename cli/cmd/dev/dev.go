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
	"strings"

	"github.com/softwarefactory-project/sf-operator/cli/cmd/dev/gerrit"
	ms "github.com/softwarefactory-project/sf-operator/cli/cmd/dev/microshift"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"

	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var devCreateAllowedArgs = []string{"gerrit", "microshift", "standalone-sf"}
var devWipeAllowedArgs = []string{"gerrit"}
var devRunTestsAllowedArgs = []string{"olm", "standalone", "upgrade"}

var microshiftUser = "cloud-user"
var defaultDiskSpace = "20G"

func createMicroshift(kmd *cobra.Command, cliCtx cliutils.SoftwareFactoryConfigContext) {
	skipLocalSetup, _ := kmd.Flags().GetBool("skip-local-setup")
	skipDeploy, _ := kmd.Flags().GetBool("skip-deploy")
	skipPostInstall, _ := kmd.Flags().GetBool("skip-post-install")
	dryRun, _ := kmd.Flags().GetBool("dry-run")
	rootDir := ms.CreateTempRootDir()

	// Arg validation
	missingArgError := errors.New("missing argument")
	msHost := cliCtx.Dev.Microshift.Host
	if msHost == "" {
		ctrl.Log.Error(missingArgError, "Host must be set in `microshift` section of the configuration")
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
		ctrl.Log.Error(missingArgError, "A valid OpenShift pull secret must be set in `microshift` section of the configuration")
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
		ctrl.Log.Error(missingArgError, "The path to the sf-operator repository must be set in `dev` section of the configuration")
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

func devCreate(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIctxOrDie(kmd, args, devCreateAllowedArgs)
	target := args[0]
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	fqdn := cliCtx.FQDN
	if target == "gerrit" {
		env := cliutils.ENV{
			Cli: cliutils.CreateKubernetesClientOrDie(kubeContext),
			Ctx: context.TODO(),
			Ns:  ns,
		}
		gerrit.EnsureGerrit(&env, fqdn)
	} else if target == "microshift" {
		createMicroshift(kmd, cliCtx)
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

func devRunTests(kmd *cobra.Command, args []string) {}

func MkDevCmd() *cobra.Command {

	var (
		deleteData        bool
		verifyCloneSSL    bool
		msSkipDeploy      bool
		msSkipLocalSetup  bool
		msSkipPostInstall bool
		msDryRun          bool
		sfResource        string
		devCmd            = &cobra.Command{
			Use:   "dev",
			Short: "development subcommands",
			Long:  "These subcommands can be used to manage a dev environment and streamline recurrent development tasks like running the operator's test suite.",
		}
		createCmd = &cobra.Command{
			Use:       "create {" + strings.Join(devCreateAllowedArgs, ", ") + "}",
			Long:      "Create a development resource. The resource can be a MicroShift cluster, a standalone SF deployment, or a gerrit instance",
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
			Use:       "runTests TESTNAME",
			Long:      "Wipe a development resource. The resource can be a gerrit instance.",
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

	devCmd.AddCommand(createCmd)
	devCmd.AddCommand(wipeCmd)
	devCmd.AddCommand(cloneAsAdminCmd)
	devCmd.AddCommand(runTestsCmd)
	return devCmd
}
