// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the main sfconfig CLI for the end user.
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/gerrit"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/nodepool"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/sfprometheus"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
	controllers "github.com/softwarefactory-project/sf-operator/controllers"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Run(erase bool) {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{Development: true})))
	sfconfig := GetConfigOrDie()
	fmt.Println("sfconfig started with: ", sfconfig)
	cli, err := utils.CreateKubernetesClient("")
	if err != nil {
		cli = EnsureCluster(err)
	}
	env := utils.ENV{
		Ctx: context.TODO(),
		Ns:  "sf",
		Cli: cli,
	}
	if erase {
		fmt.Println("Erasing...")
		// TODO: remove the sfconfig resource and the pv
	} else {
		// TODO: only do gerrit when provision demo is on?
		gerrit.EnsureGerrit(&env, sfconfig.FQDN)
		sfprometheus.EnsurePrometheus(&env, sfconfig.FQDN)
		EnsureDemoConfig(&env, &sfconfig)
		nodepool.CreateNamespaceForNodepool(&env, "", "nodepool", "")
		EnsureDeployement(&env, &sfconfig)
	}
}

// The goal of this function is to prepare a demo config
func EnsureDemoConfig(env *utils.ENV, sfconfig *Config) {
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

	template_data_output, _ := yaml.Marshal(tenantData)

	if err := os.WriteFile(tenantFile, []byte(template_data_output), 0644); err != nil {
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

func EnsureRepo(sfconfig *Config, apiKey string, name string) {
	path := filepath.Join("deploy", name)
	origin := fmt.Sprintf("https://admin:%s@gerrit.sftests.com/a/%s", apiKey, name)
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

// The goal of this function is to recorver from client creation error
func EnsureCluster(err error) client.Client {
	// TODO: perform openstack server reboot?
	panic(fmt.Errorf("cluster error: %s", err))
}

// The goal of this function is to ensure a deployment is running.
func EnsureDeployement(env *utils.ENV, sfconfig *Config) {
	fmt.Println("[+] Checking SF resource...")
	sf, err := utils.GetSF(env, "my-sf")
	if sf.Status.Ready {
		// running the operator should be a no-op
		RunOperator()

		fmt.Println("Software Factory is already ready!")
		// TODO: connect to the Zuul API and ensure it is running
		fmt.Println("Check https://zuul." + sf.Spec.FQDN)
		os.Exit(0)

	} else if err != nil {
		if errors.IsNotFound(err) {
			// The resource does not exist
			EnsureNamespacePermissions(env)
			EnsureCR(env, sfconfig)
			EnsureCertManager(env)
			EnsurePrometheusOperator(env)
			RunOperator()

		} else if utils.IsCRDMissing(err) {
			// The resource definition does not exist
			EnsureNamespacePermissions(env)
			EnsureCRD()
			EnsureCR(env, sfconfig)
			EnsureCertManager(env)
			EnsurePrometheusOperator(env)
			RunOperator()

		} else {
			// TODO: check what is the actual error and suggest counter measure, for example:
			// if microshift host is up but service is done, apply the ansible-microshift-role
			// if kubectl is not connecting ask for reboot or rebuild
			fmt.Printf("Error %v\n", errors.IsInvalid(err))
			fmt.Println(err)
		}

	} else {
		// Software Factory resource exists, but it is not ready
		if IsOperatorRunning() {
			// TODO: check operator status
			// TODO: check cluster status and/or suggest sf resource delete/recreate
		} else {
			EnsureCertManager(env)
			EnsurePrometheusOperator(env)
			RunOperator()
		}
	}

	// TODO: suggest sfconfig --erase if the command does not succeed.
	fmt.Println("[+] Couldn't deploy your software factory, sorry!")
}

func EnsureNamespacePermissions(env *utils.ENV) {
	// TODO: implement natively
	utils.RunCmd("kubectl", "label", "--overwrite", "ns", env.Ns, "pod-security.kubernetes.io/enforce=privileged")
	utils.RunCmd("kubectl", "label", "--overwrite", "ns", env.Ns, "pod-security.kubernetes.io/enforce-version=v1.24")
	utils.RunCmd("oc", "adm", "policy", "add-scc-to-user", "privileged", "-z", "default")
}

func EnsureCR(env *utils.ENV, sfconfig *Config) {
	fmt.Println("[+] Installing CR...")
	var cr sfv1.SoftwareFactory
	cr.SetName("my-sf")
	cr.SetNamespace(env.Ns)
	cr.Spec.FQDN = sfconfig.FQDN
	cr.Spec.ConfigLocation = sfv1.ConfigLocationSpec{
		BaseURL:            "http://gerrit-httpd/",
		Name:               "config",
		ZuulConnectionName: "gerrit",
	}
	cr.Spec.Zuul.GerritConns = []sfv1.GerritConnection{
		{
			Name:     "gerrit",
			Username: "zuul",
			Hostname: "gerrit-sshd",
			Puburl:   "https://gerrit.sftests.com",
		},
	}
	cr.Spec.StorageClassName = "topolvm-provisioner"
	logserverVolumeSize, _ := resource.ParseQuantity("2Gi")
	cr.Spec.Logserver.Storage.Size = logserverVolumeSize
	var err error
	for i := 0; i < 10; i++ {
		err = env.Cli.Create(env.Ctx, &cr)
		if err == nil {
			return
		}
		// Sometime the api needs a bit of time to register the CRD
		time.Sleep(2 * time.Second)
	}
	panic(fmt.Errorf("Could not install CR: %s", err))
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
		time.Sleep(2 * time.Second)
	}
	panic("cert-manager didn't become ready")
}

func EnsurePrometheusOperator(env *utils.ENV) {
	fmt.Println("[+] Installing prometheus-operator...")
	err := sfprometheus.EnsurePrometheusOperator(env)
	if err != nil {
		panic(fmt.Errorf("Could not install prometheus-operator: %s", err))
	}
}

func RunOperator() {
	fmt.Println("[+] Running the operator...")
	controllers.Main("sf", ":8081", ":8080", false, true)
}

func IsOperatorRunning() bool {
	// TODO: implement
	return false
}
