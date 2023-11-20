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

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/gerrit"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/nodepool"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/sfprometheus"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/config"
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
	Run:   func(cmd *cobra.Command, args []string) { Run() },
}

func init() {
	DevCmd.AddCommand(DevPrepareCmd)
}

func Run() {
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
	// TODO: only do gerrit when provision demo is on?
	EnsureNamespaces(&env)
	EnsureMicroshiftWorkarounds(&env)
	EnsureCertManager(&env)
	EnsurePrometheusOperator(&env)
	gerrit.EnsureGerrit(&env, sfconfig.FQDN)
	EnsureGerritAccess(sfconfig.FQDN)
	sfprometheus.EnsurePrometheus(&env, sfconfig.FQDN, false)
	EnsureDemoConfig(&env, &sfconfig)
	nodepool.CreateNamespaceForNodepool(&env, "", "nodepool", "")
	EnsureCRD()
}

// EnsureDemoConfig prepares a demo config
func EnsureDemoConfig(env *utils.ENV, sfconfig *config.SFConfig) {
	fmt.Println("[+] Ensuring demo config")
	apiKey := string(utils.GetSecret(env, "gerrit-admin-api-key"))
	EnsureRepo(sfconfig, apiKey, "config")
	EnsureRepo(sfconfig, apiKey, "demo-project")
	SetupTenant("deploy/config", "demo-tenant")
	PushRepoIfNeeded("deploy/config")
}

func SetupTenant(configPath string, tenantName string) {
	tenantDir := filepath.Join(configPath, "zuul")
	if err := os.MkdirAll(tenantDir, 0755); err != nil {
		panic(err)
	}
	tenantFile := filepath.Join(tenantDir, "main.yaml")

	tenantData := utils.TenantConfig{
		{
			Tenant: utils.TenantBody{
				Name: tenantName,
				Source: utils.TenantConnectionSource{
					"gerrit": {
						ConfigProjects:    []string{"config"},
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

func EnsureRepo(sfconfig *config.SFConfig, apiKey string, name string) {
	path := filepath.Join("deploy", name)
	origin := fmt.Sprintf("https://admin:%s@gerrit.%s/a/%s", apiKey, sfconfig.FQDN, name)
	if _, err := os.Stat(filepath.Join(path, ".git")); os.IsNotExist(err) {
		utils.RunCmd("git", "-c", "http.sslVerify=false", "clone", origin, path)
	} else {
		utils.RunCmd("git", "-C", path, "remote", "set-url", "origin", origin)
		utils.RunCmd("git", "-C", path, "fetch", "origin")
	}
	utils.RunCmd("git", "-C", path, "config", "http.sslverify", "false")
	utils.RunCmd("git", "-C", path, "config", "user.email", "admin@"+sfconfig.FQDN)
	utils.RunCmd("git", "-C", path, "config", "user.name", "admin")
	utils.RunCmd("git", "-C", path, "reset", "--hard", "origin/master")
}

func EnsureNamespaces(env *utils.ENV) {
	// TODO: implement natively
	utils.EnsureNamespace(env, env.Ns)
	utils.RunCmd("kubectl", "config", "set-context", "microshift", "--namespace="+env.Ns)
	utils.RunCmd("kubectl", "label", "--overwrite", "ns", env.Ns, "pod-security.kubernetes.io/enforce=privileged")
	utils.RunCmd("kubectl", "label", "--overwrite", "ns", env.Ns, "pod-security.kubernetes.io/enforce-version=v1.24")
	utils.RunCmd("oc", "adm", "policy", "add-scc-to-user", "privileged", "-z", "default")

	utils.EnsureNamespace(env, "operators")
	utils.RunCmd("oc", "adm", "policy", "add-scc-to-user", "privileged", "system:serviceaccount:operators:default")
}

func EnsureMicroshiftWorkarounds(env *utils.ENV) {
	// TODO: migrate from Makefile to here
	utils.RunCmd("make", "setup-prometheus-operator-serviceaccount", "OPERATOR_NAMESPACE=operators")
}

func EnsureCRD() {
	// TODO: implement natively and avoir re-entry
	fmt.Println("[+] Installing CRD...")
	utils.RunMake("install")
}

func EnsureCertManager(env *utils.ENV) {
	// TODO: implement natively
	fmt.Println("[+] Installing Cert-Manager...")
	utils.RunMake("install-cert-manager")
	// Mitigate the issue
	// failed calling webhook "mutate.webhooks.cert-manager.io": failed to call webhook: Post "https://cert-manager-webhook-service.operators.svc:443/mutate?timeout=10s": no endpoints available for service "cert-manager-webhook-service"
	fmt.Println("[+] Waiting for Cert-Manager")
	for i := 0; i < 10; i++ {
		if utils.IsCertManagerRunning(env) {
			return
		}
		time.Sleep(6 * time.Second)
	}
	panic("cert-manager didn't become ready")
}

func EnsurePrometheusOperator(env *utils.ENV) {
	fmt.Println("[+] Installing prometheus-operator...")
	err := sfprometheus.EnsurePrometheusOperator(env)
	if err != nil {
		panic(fmt.Errorf("could not install prometheus-operator: %s", err))
	}
}
