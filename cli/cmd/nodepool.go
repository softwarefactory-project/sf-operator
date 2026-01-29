// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

package cmd

/*
"nodepool" subcommands can be used to interact with and configure the Nodepool component of a SF deployment.
*/

import (
	"bytes"
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
	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

var npGetAllowedArgs = []string{"builder-ssh-key"}
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
	cliCtx := cliutils.GetCLIContext(kmd)
	target := args[0]
	if target == "builder-ssh-key" {
		pubKey, _ := kmd.Flags().GetString("pubkey")
		getBuilderSSHKey(cliCtx, pubKey)
	}
}

func npCreate(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIContext(kmd)
	if args[0] == "openshiftpods-namespace" {
		nodepoolContext, _ := kmd.Flags().GetString("nodepool-context")
		nodepoolNamespace, _ := kmd.Flags().GetString("nodepool-namespace")
		showConfigTemplate, _ := kmd.Flags().GetBool("show-config-template")
		skipProvidersSecrets, _ := kmd.Flags().GetBool("skip-providers-secrets")

		CreateNamespaceForNodepool(cliCtx, nodepoolContext, nodepoolNamespace, skipProvidersSecrets)
		if showConfigTemplate {
			configTemplate := mkNodepoolOpenshiftPodsConfigTemplate(nodepoolNamespace)
			fmt.Println("Nodepool configuration template:")
			fmt.Println(configTemplate)
		}
	}
}

func CreateNamespaceForNodepool(sfEnv *controllers.SFKubeContext, nodepoolContext, nodepoolNamespace string, skipProvidersSecrets bool) {
	npEnv, err := controllers.MkSFKubeContext("", nodepoolNamespace, nodepoolContext)
	if err != nil {
		logging.LogE(err, "Could not create nodepool kube client")
		os.Exit(1)
	}

	npEnv.EnsureNamespaceOrDie(nodepoolNamespace)
	npEnv.EnsureServiceAccountOrDie(nodepoolServiceAccount)
	ensureNodepoolRole(&npEnv)
	token := ensureNodepoolServiceAccountSecret(&npEnv)
	npKubeConfig := createNodepoolKubeConfigOrDie(nodepoolContext, nodepoolNamespace, token)
	kconfig, err := clientcmd.Write(npKubeConfig)

	if err != nil {
		ctrl.Log.Error(err, "Could not serialize nodepool's kubeconfig")
	}
	if skipProvidersSecrets {
		fmt.Println("Provider kubeconfig:")
		fmt.Println(string(kconfig))
	} else {
		ensureNodepoolKubeProvidersSecrets(sfEnv, kconfig)
	}

}

func ensureNodepoolKubeProvidersSecrets(env *controllers.SFKubeContext, kubeconfig []byte) {

	var secret apiv1.Secret
	if !env.GetM(controllers.NodepoolProvidersSecretsName, &secret) {
		// Initialize the secret data
		secret.Name = controllers.NodepoolProvidersSecretsName
		secret.Data = make(map[string][]byte)
		if kubeconfig != nil {
			secret.Data["kube.config"] = kubeconfig
		}
		env.CreateROrDie(&secret)
	} else {
		// Handle secret update
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		needUpdate := false
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
			env.UpdateROrDie(&secret)
		} else {
			ctrl.Log.Info("Secret \"" + controllers.NodepoolProvidersSecretsName + "\" already up to date, doing nothing")
		}
	}
}

func getBuilderSSHKey(sfEnv *controllers.SFKubeContext, pubKey string) {
	var secret apiv1.Secret
	if sfEnv.GetM("nodepool-builder-ssh-key", &secret) {
		if pubKey == "" {
			fmt.Println(string(secret.Data["pub"]))
		} else {
			os.WriteFile(pubKey, secret.Data["pub"], 0600)
			ctrl.Log.Info("File " + pubKey + " saved")
		}
	} else {
		ctrl.Log.Error(errors.New("Secret nodepool-builder-ssh-key not found in namespace "+sfEnv.Ns),
			"Error fetching builder SSH key")
		os.Exit(1)
	}
}

func ensureNodepoolRole(env *controllers.SFKubeContext) {
	var role rbacv1.Role
	var roleBinding rbacv1.RoleBinding

	if !env.GetM(nodepoolRole, &role) {
		role.Name = nodepoolRole
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/exec", "pods/portforward", "services", "persistentvolumeclaims", "configmaps", "secrets"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		}
		env.CreateROrDie(&role)
	}

	if !env.GetM(nodepoolRoleBinding, &roleBinding) {
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
		env.CreateROrDie(&roleBinding)
	}
}

func ensureNodepoolServiceAccountSecret(env *controllers.SFKubeContext) string {
	var secret apiv1.Secret
	if !env.GetM(nodepoolToken, &secret) {
		secret.Name = nodepoolToken
		secret.ObjectMeta.Annotations = map[string]string{
			"kubernetes.io/service-account.name": nodepoolServiceAccount,
		}
		secret.Type = "kubernetes.io/service-account-token"
		env.CreateROrDie(&secret)
	}
	var token []byte
	for retry := 1; retry < 20; retry++ {
		token = secret.Data["token"]
		if token != nil {
			break
		}
		time.Sleep(time.Second)
		env.GetM(nodepoolToken, &secret)
	}
	if token == nil {
		ctrl.Log.Error(errors.New("query timeout"), "Error getting nodepool service account token")
		os.Exit(1)
	}
	return string(token)
}

func createNodepoolKubeConfigOrDie(contextName string, ns string, token string) cliapi.Config {
	ctx, err := controllers.MkSFKubeContext("", ns, contextName)
	if err != nil {
		logging.LogE(err, "Could not build kube context")
		os.Exit(1)
	}
	currentConfig := ctx.RESTConfig
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
		createCmd, _, getCmd = cliutils.GetCRUDSubcommands()
	)

	getCmd.Run = npGet
	getCmd.Use = "get {builder-ssh-key}"
	getCmd.Long = "Get a Nodepool resource. The resource can be the providers secrets or the builder's public SSH key."
	getCmd.ValidArgs = npGetAllowedArgs
	getCmd.Flags().StringVar(&builderPubKey, "pubkey", "", "(use with builder-ssh-key) File where to dump nodepool-builder's SSH public key")

	createCmd.Run = npCreate
	createCmd.Use = "create {openshiftpods-namespace}"
	createCmd.Long = "Create a nodepool resource. The resource can be: a namespace that can be used with the \"openshiftpods\" provider."
	createCmd.ValidArgs = npCreateAllowedArgs
	createCmd.Flags().StringVar(&nodepoolContext, "nodepool-context", "", "(openshiftpods-namespace) the kube context nodepool will use to configure the namespace")
	createCmd.Flags().StringVar(&nodepoolNamespace, "nodepool-namespace", "nodepool", "(openshiftpods-namespace) the name of the namespace to create")
	createCmd.Flags().BoolVar(&showConfigTemplate, "show-config-template", false, "(openshiftpods-namespace) display a YAML snippet that can be used to configure an \"openshiftpods\" provider with nodepool")
	createCmd.Flags().BoolVar(&skipProvidersSecrets, "skip-providers-secrets", false, "openshiftpods-namespace) do not update providers secrets, and instead display the nodepool kube config on stdout")

	nodepoolCmd.AddCommand(createCmd)
	nodepoolCmd.AddCommand(getCmd)
	return nodepoolCmd
}
