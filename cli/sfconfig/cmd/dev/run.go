// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the main sfconfig CLI for the end user.
// The goal is to be a onestop shop to get the service running with a single `sfconfig` command invocation.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	cligerrit "github.com/softwarefactory-project/sf-operator/cli/cmd/dev/gerrit"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"

	bootstraptenantconfigrepo "github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/bootstrap-tenant-config-repo"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/nodepool"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/sfprometheus"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/config"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/zuulcf"
	"github.com/spf13/cobra"
)

var DevCmd = &cobra.Command{
	Use:   "dev",
	Short: "developer utilities",
	Run:   func(cmd *cobra.Command, args []string) {},
}

var DevPrepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "prepare dev environment",
	Run:   func(cmd *cobra.Command, args []string) { Run(cmd) },
}

func init() {
	var installPrometheus bool
	var dontUpdateDemoTenantDefinition bool
	DevPrepareCmd.Flags().BoolVar(
		&installPrometheus, "with-prometheus", false,
		"Add this flag to spin a prometheus instance as well")
	DevPrepareCmd.Flags().BoolVar(
		&dontUpdateDemoTenantDefinition, "dont-update-demo-tenant", false,
		"Add this flag to avoid reseting demo-tenant tenant definition")
	DevCmd.AddCommand(DevPrepareCmd)
}

func Run(cmd *cobra.Command) {
	withPrometheus, _ := cmd.Flags().GetBool("with-prometheus")
	dontUpdateDemoTenantDefinition, _ := cmd.Flags().GetBool("dont-update-demo-tenant")
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{Development: true})))
	sfconfig := config.GetSFConfigOrDie()
	fmt.Println("sfconfig started with: ", sfconfig)
	cli, err := utils.CreateKubernetesClient("")
	if err != nil {
		panic(err)
	}
	env := utils.ENV{
		Ctx: context.TODO(),
		Ns:  "sf",
		Cli: cli,
	}
	gEnv := cliutils.ENV{
		Cli: cli,
		Ns:  "sf",
		Ctx: context.TODO(),
	}
	// TODO: only do gerrit when provision demo is on?
	cligerrit.EnsureGerrit(&gEnv, sfconfig.FQDN)
	EnsureGerritAccess(sfconfig.FQDN)
	if withPrometheus {
		sfprometheus.EnsurePrometheus(&env, sfconfig.FQDN, false)
	}
	EnsureDemoConfig(&env, &sfconfig, !dontUpdateDemoTenantDefinition)
	nodepool.CreateNamespaceForNodepool(&env, "", "nodepool", "")
	EnsureCRD()
}

// EnsureDemoConfig prepares a demo config
func EnsureDemoConfig(env *utils.ENV, sfconfig *config.SFConfig, updateDemoTenantDefinition bool) {
	var (
		configRepoPath     = "deploy/config"
		demoConfigRepoPath = "deploy/demo-tenant-config"
	)
	gEnv := cliutils.ENV{
		Cli: env.Cli,
		Ns:  env.Ns,
		Ctx: env.Ctx,
	}
	fmt.Println("[+] Ensuring demo config")
	for _, repo := range []string{
		"config", "demo-tenant-config", "demo-project",
	} {
		path := filepath.Join("deploy", repo)
		cligerrit.CloneAsAdmin(&gEnv, sfconfig.FQDN, repo, path, false)
	}
	setupDemoTenantConfigRepo(demoConfigRepoPath)
	PushRepoIfNeeded(demoConfigRepoPath)
	if updateDemoTenantDefinition {
		SetupTenantInMainYAMLFile(configRepoPath, "demo-tenant")
		PushRepoIfNeeded(configRepoPath)
	}
}

func setupDemoTenantConfigRepo(configPath string) {
	bootstraptenantconfigrepo.InitConfigRepo(
		"gerrit", "gerrit", configPath)
	utils.RunCmd("git", "-C", configPath, "add", "zuul.d/", "playbooks/")
}

func SetupTenantInMainYAMLFile(configPath string, tenantName string) {
	tenantDir := filepath.Join(configPath, "zuul")
	if err := os.MkdirAll(tenantDir, 0755); err != nil {
		panic(err)
	}
	tenantFile := filepath.Join(tenantDir, "main.yaml")

	tenantData := zuulcf.TenantConfig{
		{
			Tenant: zuulcf.TenantBody{
				Name: tenantName,
				Source: zuulcf.TenantConnectionSource{
					"opendev.org": {
						UntrustedProjects: []string{"zuul/zuul-jobs"},
					},
					"gerrit": {
						ConfigProjects:    []string{"demo-tenant-config"},
						UntrustedProjects: []string{"demo-project"},
					},
				},
			},
		},
	}

	templateDataOutput, _ := yaml.Marshal(tenantData)

	if err := os.WriteFile(tenantFile, []byte(templateDataOutput), 0644); err != nil {
		panic(err)
	}
	utils.RunCmd("git", "-C", configPath, "add", "zuul/main.yaml")
}

func PushRepoIfNeeded(path string) {
	out, err := exec.Command("git", "-C", path, "status", "--porcelain").Output()
	if err != nil {
		panic(err)
	}
	if len(out) > 0 {
		fmt.Println("[+] Pushing new config...")
		utils.RunCmd("git", "-C", path, "commit", "-m", "Automatic update", "-a")
		utils.RunCmd("git", "-C", path, "push", "origin")
	}
}

func EnsureGerritAccess(fqdn string) {
	fmt.Println("[+] Wait for Gerrit reachable via the Route ...")
	params := []string{"--fail", "-k", fmt.Sprintf("https://gerrit.%s/projects/", fqdn)}
	delay := 20 * time.Second
	attempts := 0
	for {
		if attempts >= 3 {
			panic("Unable to access Gerrit via the Route !")
		}
		err := utils.RunCmdNoPanic("curl", params...)
		if err != nil {
			attempts += 1
			fmt.Println("Gerrit not available via the Route. Retrying in", delay.String(), "seconds ...")
			time.Sleep(delay)
		} else {
			fmt.Println("Gerrit available via the Route. Continue")
			break
		}
	}
}

func EnsureCRD() {
	// TODO: implement natively and avoir re-entry
	fmt.Println("[+] Installing CRD...")
	utils.RunMake("install")
}
