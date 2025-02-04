// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

package cmd

/*
"nodepool" subcommands can be used to interact with and configure the Nodepool component of a SF deployment.
*/

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	cliapi "k8s.io/client-go/tools/clientcmd/api"

	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

var npGetAllowedArgs = []string{"providers-secrets", "builder-ssh-key"}
var npConfigureAllowedArgs = []string{"providers-secrets"}
var npCreateAllowedArgs = []string{"openshiftpods-namespace"}

// openshiftpods namespace default values
var (
	nodepoolServiceAccount = "nodepool-sa"
	nodepoolRole           = "nodepool-role"
	nodepoolRoleBinding    = "nodepool-rb"
	nodepoolToken          = "nodepool-token"
	nodepoolKubeContext    = "openshiftpods"
)

func npGet(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIctxOrDie(kmd, args, npGetAllowedArgs)
	target := args[0]
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	if target == "providers-secrets" {
		cloudsFile, _ := kmd.Flags().GetString("clouds")
		if cloudsFile == "" {
			cloudsFile = cliCtx.Components.Nodepool.CloudsFile
		}
		kubeFile, _ := kmd.Flags().GetString("kube")
		if kubeFile == "" {
			kubeFile = cliCtx.Components.Nodepool.KubeFile
		}
		getProvidersSecret(ns, kubeContext, cloudsFile, kubeFile)
	}
	if target == "builder-ssh-key" {
		pubKey, _ := kmd.Flags().GetString("pubkey")
		getBuilderSSHKey(ns, kubeContext, pubKey)
	}
}

func npConfigure(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIctxOrDie(kmd, args, npConfigureAllowedArgs)
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	cloudsFile, _ := kmd.Flags().GetString("clouds")
	if cloudsFile == "" {
		cloudsFile = cliCtx.Components.Nodepool.CloudsFile
	}
	kubeFile, _ := kmd.Flags().GetString("kube")
	if kubeFile == "" {
		kubeFile = cliCtx.Components.Nodepool.KubeFile
	}
	if cloudsFile == "" && kubeFile == "" {
		ctrl.Log.Error(errors.New("not enough parameters"),
			"a clouds.yaml file or a kube.config file must be passed to the command via the --clouds or --kube arguments")
		os.Exit(1)
	}
	cloudsContent, err := cliutils.GetFileContent(cloudsFile)
	if err != nil {
		ctrl.Log.Error(err, "Error opening %s", cloudsFile)
		os.Exit(1)
	}
	kubeContent, err := cliutils.GetFileContent(kubeFile)
	if err != nil {
		ctrl.Log.Error(err, "Error opening %s", kubeFile)
		os.Exit(1)
	}
	client := cliutils.CreateKubernetesClientOrDie(kubeContext)
	ctx := context.TODO()
	env := cliutils.ENV{
		Cli:         client,
		Ctx:         ctx,
		Ns:          ns,
		IsOpenShift: controllers.CheckOpenShift(),
	}
	ensureNodepoolProvidersSecrets(&env, cloudsContent, kubeContent)
}

func npCreate(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIctxOrDie(kmd, args, npCreateAllowedArgs)
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	client := cliutils.CreateKubernetesClientOrDie(kubeContext)
	ctx := context.TODO()
	sfEnv := cliutils.ENV{
		Cli:         client,
		Ctx:         ctx,
		Ns:          ns,
		IsOpenShift: controllers.CheckOpenShift(),
	}
	if args[0] == "openshiftpods-namespace" {
		nodepoolContext, _ := kmd.Flags().GetString("nodepool-context")
		nodepoolNamespace, _ := kmd.Flags().GetString("nodepool-namespace")
		showConfigTemplate, _ := kmd.Flags().GetBool("show-config-template")
		skipProvidersSecrets, _ := kmd.Flags().GetBool("skip-providers-secrets")

		if nodepoolContext == kubeContext {
			ctrl.Log.Info("Warning: Nodepool will use the same cluster context as SF")
			if nodepoolNamespace == ns {
				ctrl.Log.Info("Warning: Nodepool will manage resources in the same namespace as the Software Factory deployment")
			}
		}
		CreateNamespaceForNodepool(&sfEnv, nodepoolContext, nodepoolNamespace, skipProvidersSecrets)
		if showConfigTemplate {
			configTemplate := mkNodepoolOpenshiftPodsConfigTemplate(nodepoolNamespace)
			fmt.Println("Nodepool configuration template:")
			fmt.Println(configTemplate)
		}
	}
}

func CreateNamespaceForNodepool(sfEnv *cliutils.ENV, nodepoolContext, nodepoolNamespace string, skipProvidersSecrets bool) {
	client := cliutils.CreateKubernetesClientOrDie(nodepoolContext)
	ctx := context.TODO()
	nodepoolEnv := cliutils.ENV{
		Cli:         client,
		Ctx:         ctx,
		Ns:          nodepoolNamespace,
		IsOpenShift: controllers.CheckOpenShift(),
	}

	cliutils.EnsureNamespaceOrDie(&nodepoolEnv, nodepoolNamespace)
	cliutils.EnsureServiceAccountOrDie(&nodepoolEnv, nodepoolServiceAccount)
	ensureNodepoolRole(&nodepoolEnv)
	token := ensureNodepoolServiceAccountSecret(&nodepoolEnv)
	nodepoolKubeConfig := createNodepoolKubeConfigOrDie(nodepoolContext, nodepoolNamespace, token)
	kconfig, err := clientcmd.Write(nodepoolKubeConfig)

	if err != nil {
		ctrl.Log.Error(err, "Could not serialize nodepool's kubeconfig")
	}
	if skipProvidersSecrets {
		fmt.Println("Provider kubeconfig:")
		fmt.Println(string(kconfig))
	} else {
		ensureNodepoolProvidersSecrets(sfEnv, []byte{}, kconfig)
	}

}

func ensureNodepoolProvidersSecrets(env *cliutils.ENV, cloudconfig []byte, kubeconfig []byte) {

	var secret apiv1.Secret
	if !cliutils.GetMOrDie(env, controllers.NodepoolProvidersSecretsName, &secret) {
		// Initialize the secret data
		secret.Name = controllers.NodepoolProvidersSecretsName
		secret.Data = make(map[string][]byte)
		if cloudconfig != nil {
			secret.Data["clouds.yaml"] = cloudconfig
		}
		if kubeconfig != nil {
			secret.Data["kube.config"] = kubeconfig
		}
		cliutils.CreateROrDie(env, &secret)
	} else {
		// Handle secret update
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		needUpdate := false
		if cloudconfig != nil {
			if !bytes.Equal(secret.Data["clouds.yaml"], cloudconfig) {
				ctrl.Log.Info("Updating clouds config ...")
				secret.Data["clouds.yaml"] = cloudconfig
				needUpdate = true
			}
		} else {
			if _, ok := secret.Data["clouds.yaml"]; ok {
				ctrl.Log.Info("Removing clouds config ...")
				delete(secret.Data, "clouds.yaml")
				needUpdate = true
			}
		}
		if kubeconfig != nil {
			if !bytes.Equal(secret.Data["kube.config"], kubeconfig) {
				ctrl.Log.Info("Updating the kube config ...")
				secret.Data["kube.config"] = kubeconfig
				needUpdate = true
			}
		} else {
			if _, ok := secret.Data["kube.config"]; ok {
				ctrl.Log.Info("Removing the kube config ...")
				delete(secret.Data, "kube.config")
				needUpdate = true
			}
		}
		if needUpdate {
			cliutils.UpdateROrDie(env, &secret)
		} else {
			ctrl.Log.Info("Secret \"" + controllers.NodepoolProvidersSecretsName + "\" already up to date, doing nothing")
		}
	}
}

func getProvidersSecret(ns string, kubeContext string, cloudsFile string, kubeFile string) {
	client := cliutils.CreateKubernetesClientOrDie(kubeContext)
	ctx := context.TODO()
	sfEnv := cliutils.ENV{
		Cli: client,
		Ctx: ctx,
		Ns:  ns,
	}

	var secret apiv1.Secret
	if cliutils.GetMOrDie(&sfEnv, controllers.NodepoolProvidersSecretsName, &secret) {
		if len(secret.Data["clouds.yaml"]) > 0 {
			if cloudsFile == "" {
				fmt.Println("clouds.yaml:")
				fmt.Println(string(secret.Data["clouds.yaml"]))
			} else {
				// TODO before we write to file, we should ensure the file, if it exists, is older than
				// the upstream secret to avoid losing more recent secrets.
				os.WriteFile(cloudsFile, secret.Data["clouds.yaml"], 0600)
				ctrl.Log.Info("File " + cloudsFile + " updated")
			}
		}
		if len(secret.Data["kube.config"]) > 0 {
			if kubeFile == "" {
				fmt.Println("kube.config:")
				fmt.Println(string(secret.Data["kube.config"]))
			} else {
				os.WriteFile(kubeFile, secret.Data["kube.config"], 0644)
				ctrl.Log.Info("File " + kubeFile + " updated")
			}
		}
	} else {
		ctrl.Log.Error(errors.New("Secret "+controllers.NodepoolProvidersSecretsName+" not found in namespace "+ns),
			"Error fetching providers secrets")
		os.Exit(1)
	}
}

func getBuilderSSHKey(ns string, kubeContext string, pubKey string) {
	sfEnv := cliutils.ENV{
		Cli: cliutils.CreateKubernetesClientOrDie(kubeContext),
		Ctx: context.TODO(),
		Ns:  ns,
	}
	var secret apiv1.Secret
	if cliutils.GetMOrDie(&sfEnv, "nodepool-builder-ssh-key", &secret) {
		if pubKey == "" {
			fmt.Println(string(secret.Data["pub"]))
		} else {
			os.WriteFile(pubKey, secret.Data["pub"], 0600)
			ctrl.Log.Info("File " + pubKey + " saved")
		}
	} else {
		ctrl.Log.Error(errors.New("Secret nodepool-builder-ssh-key not found in namespace "+ns),
			"Error fetching builder SSH key")
		os.Exit(1)
	}
}

func ensureNodepoolRole(env *cliutils.ENV) {
	var role rbacv1.Role
	var roleBinding rbacv1.RoleBinding

	if !cliutils.GetMOrDie(env, nodepoolRole, &role) {
		role.Name = nodepoolRole
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/exec", "services", "persistentvolumeclaims", "configmaps", "secrets"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		}
		cliutils.CreateROrDie(env, &role)
	}

	if !cliutils.GetMOrDie(env, nodepoolRoleBinding, &roleBinding) {
		roleBinding.Name = nodepoolRoleBinding
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: nodepoolServiceAccount,
			},
		}
		roleBinding.RoleRef.Kind = "Role"
		roleBinding.RoleRef.Name = nodepoolRole
		roleBinding.RoleRef.APIGroup = "rbac.authorization.k8s.io"
		cliutils.CreateROrDie(env, &roleBinding)
	}
}

func ensureNodepoolServiceAccountSecret(env *cliutils.ENV) string {
	var secret apiv1.Secret
	if !cliutils.GetMOrDie(env, nodepoolToken, &secret) {
		secret.Name = nodepoolToken
		secret.ObjectMeta.Annotations = map[string]string{
			"kubernetes.io/service-account.name": nodepoolServiceAccount,
		}
		secret.Type = "kubernetes.io/service-account-token"
		cliutils.CreateROrDie(env, &secret)
	}
	var token []byte
	for retry := 1; retry < 20; retry++ {
		token = secret.Data["token"]
		if token != nil {
			break
		}
		time.Sleep(time.Second)
		cliutils.GetMOrDie(env, nodepoolToken, &secret)
	}
	if token == nil {
		ctrl.Log.Error(errors.New("query timeout"), "Error getting nodepool service account token")
		os.Exit(1)
	}
	return string(token)
}

func createNodepoolKubeConfigOrDie(contextName string, ns string, token string) cliapi.Config {
	currentConfig := controllers.GetConfigContextOrDie(contextName)
	if strings.HasPrefix(currentConfig.Host, "https://localhost") || strings.HasPrefix(currentConfig.Host, "https://127.") {
		ctrl.Log.Error(
			errors.New("invalid config host address"),
			"The server address of the context used by nodepool cannot be \"localhost\" and must be resolvable from nodepool's pod.",
		)
		os.Exit(1)
	}
	return cliapi.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: map[string]*cliapi.Cluster{
			"OpenshiftPodsCluster": {
				Server:                   currentConfig.Host + currentConfig.APIPath,
				CertificateAuthorityData: currentConfig.TLSClientConfig.CAData,
			},
		},
		Contexts: map[string]*cliapi.Context{
			nodepoolKubeContext: {
				Cluster:   "OpenshiftPodsCluster",
				Namespace: ns,
				AuthInfo:  nodepoolServiceAccount,
			},
		},
		CurrentContext: nodepoolKubeContext,
		AuthInfos: map[string]*cliapi.AuthInfo{
			nodepoolServiceAccount: {
				Token: token,
			},
		},
	}
}

func mkNodepoolOpenshiftPodsConfigTemplate(nodepoolNamespace string) string {

	type Label struct {
		Name  string `json:"name"`
		Image string `json:"image"`
	}
	type Pool struct {
		Name   string  `json:"name"`
		Labels []Label `json:"labels"`
	}
	type Provider struct {
		Name    string `json:"name"`
		Driver  string `json:"driver"`
		Context string `json:"context"`
		Pools   []Pool `json:"pools"`
	}
	type ProvidersConfig struct {
		Providers []Provider `json:"providers"`
	}
	templateConfig := ProvidersConfig{
		Providers: []Provider{
			{
				Name:    "openshiftpods",
				Driver:  "openshiftpods",
				Context: nodepoolKubeContext,
				Pools: []Pool{
					{
						Name: nodepoolNamespace,
						Labels: []Label{
							{
								Name:  "fedora-latest",
								Image: "quay.io/fedora/fedora:latest",
							},
						},
					},
				},
			},
		},
	}
	templateYaml, err := yaml.Marshal(templateConfig)
	if err != nil {
		ctrl.Log.Error(err, "Could not serialize sample provider configuration")
		os.Exit(1)
	}
	return string(templateYaml)
}

func MkNodepoolCmd() *cobra.Command {

	var (
		cloudsOutput         string
		kubeconfigOutput     string
		builderPubKey        string
		nodepoolContext      string
		nodepoolNamespace    string
		showConfigTemplate   bool
		skipProvidersSecrets bool

		nodepoolCmd = &cobra.Command{
			Use:   "nodepool",
			Short: "Nodepool subcommands",
			Long:  `These subcommands can be used to interact with the Nodepool component of a Software Factory deployment.`,
		}
		createCmd, configureCmd, getCmd = cliutils.GetCRUDSubcommands()
	)

	getCmd.Run = npGet
	getCmd.Use = "get {providers-secrets, builder-ssh-key}"
	getCmd.Long = "Get a Nodepool resource. The resource can be the providers secrets or the builder's public SSH key."
	getCmd.ValidArgs = npGetAllowedArgs
	getCmd.Flags().StringVar(&cloudsOutput, "clouds", "", "(use with providers-secrets) File where to dump the clouds secrets")
	getCmd.Flags().StringVar(&kubeconfigOutput, "kube", "", "(use with providers-secrets) File where to dump the kube secrets")
	getCmd.Flags().StringVar(&builderPubKey, "pubkey", "", "(use with builder-ssh-key) File where to dump nodepool-builder's SSH public key")

	configureCmd.Run = npConfigure
	configureCmd.Use = "configure {providers-secrets}"
	configureCmd.Long = "Configure OpenStack and/or K8s-based providers' secrets from local files."
	configureCmd.ValidArgs = npConfigureAllowedArgs
	configureCmd.Flags().StringVar(&cloudsOutput, "clouds", "", "(use with providers-secrets) File to read the clouds secrets from")
	configureCmd.Flags().StringVar(&kubeconfigOutput, "kube", "", "(use with providers-secrets) File to read the kube secrets from")

	createCmd.Run = npCreate
	createCmd.Use = "create {openshiftpods-namespace}"
	createCmd.Long = "Create a nodepool resource. The resource can be: a namespace that can be used with the \"openshiftpods\" provider."
	createCmd.ValidArgs = npCreateAllowedArgs
	createCmd.Flags().StringVar(&nodepoolContext, "nodepool-context", "", "(openshiftpods-namespace) the kube context nodepool will use to configure the namespace")
	createCmd.Flags().StringVar(&nodepoolNamespace, "nodepool-namespace", "nodepool", "(openshiftpods-namespace) the name of the namespace to create")
	createCmd.Flags().BoolVar(&showConfigTemplate, "show-config-template", false, "(openshiftpods-namespace) display a YAML snippet that can be used to configure an \"openshiftpods\" provider with nodepool")
	createCmd.Flags().BoolVar(&skipProvidersSecrets, "skip-providers-secrets", false, "openshiftpods-namespace) do not update providers secrets, and instead display the nodepool kube config on stdout")

	nodepoolCmd.AddCommand(createCmd)
	nodepoolCmd.AddCommand(configureCmd)
	nodepoolCmd.AddCommand(getCmd)
	return nodepoolCmd
}
