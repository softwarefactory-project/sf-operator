/*
Copyright © 2023 Redhat
SPDX-License-Identifier: Apache-2.0
*/
package nodepool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	sf "github.com/softwarefactory-project/sf-operator/controllers"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/clientcmd"
	cliapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
)

func ensureNamespace(env *utils.ENV, name string) {
	var ns apiv1.Namespace
	if err := env.Cli.Get(env.Ctx, client.ObjectKey{Name: name}, &ns); errors.IsNotFound(err) {
		ns.Name = name
		utils.CreateR(env, &ns)
	} else if err != nil {
		panic(fmt.Errorf("Could not get namespace: %s", err))
	}
}

func ensureRole(env *utils.ENV, sa string) {
	var role rbacv1.Role
	if !utils.GetM(env, "nodepool-role", &role) {
		role.Name = "nodepool-role"
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
		utils.CreateR(env, &role)
	} else {
		// TODO: update if needed
	}

	var rb rbacv1.RoleBinding
	if !utils.GetM(env, "nodepool-rb", &rb) {
		rb.Name = "nodepool-rb"
		rb.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: sa}}
		rb.RoleRef.Kind = "Role"
		rb.RoleRef.Name = "nodepool-role"
		rb.RoleRef.APIGroup = "rbac.authorization.k8s.io"
		utils.CreateR(env, &rb)
	}
}

func ensureServiceAccountSecret(env *utils.ENV, sa string) string {
	var secret apiv1.Secret
	if !utils.GetM(env, "nodepool-token", &secret) {
		secret.Name = "nodepool-token"
		secret.ObjectMeta.Annotations = map[string]string{
			"kubernetes.io/service-account.name": sa,
		}
		secret.Type = "kubernetes.io/service-account-token"
		utils.CreateR(env, &secret)
	}
	for retry := 1; retry < 20; retry++ {
		token := secret.Data["token"]
		if token != nil {
			return string(token)
		}
		time.Sleep(time.Second)
		utils.GetM(env, "nodepool-token", &secret)
	}
	panic("Could not find token")
}

func createKubeConfig(contextName string, ns string, sa string, token string) cliapi.Config {
	currentConfig := utils.GetConfigContextOrDie(contextName)
	if strings.HasPrefix(
		currentConfig.Host,
		"https://localhost",
	) || strings.HasPrefix(
		currentConfig.Host,
		"https://127.",
	) {
		panic(fmt.Errorf(
			"The target context server address can't be localhost, please change from %s to a publicly reachable name",
			currentConfig.Host))
	}
	return cliapi.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: map[string]*cliapi.Cluster{
			"microshift": {
				Server:                   currentConfig.Host + currentConfig.APIPath,
				CertificateAuthorityData: currentConfig.TLSClientConfig.CAData,
			},
		},
		Contexts: map[string]*cliapi.Context{
			"microshift": {
				Cluster:   "microshift",
				Namespace: ns,
				AuthInfo:  sa,
			},
		},
		CurrentContext: "microshift",
		AuthInfos: map[string]*cliapi.AuthInfo{
			sa: {
				Token: token,
			},
		},
	}
}

func ensureKubeConfigSecret(env *utils.ENV, config []byte, name string) {
	var secret apiv1.Secret
	if !utils.GetM(env, name, &secret) {
		secret.Name = name
		secret.Data = map[string][]byte{"kube.config": config}
		utils.CreateR(env, &secret)
	} else {
		// TODO: update data if needed
	}
}

func CreateNamespaceForNodepool(sfEnv *utils.ENV, nodepoolContext string, nodepoolNamespace string, sfContext string) {
	nodepoolEnv := utils.ENV{Cli: sfEnv.Cli, Ctx: sfEnv.Ctx, Ns: nodepoolNamespace}
	if nodepoolContext == sfContext {
		fmt.Println("Warning: deploying nodepool resources on the same cluster as sf")
	} else {
		// We need to recreate the client
		nodepoolEnv.Cli = utils.CreateKubernetesClientOrDie(nodepoolContext)
	}
	sa := "nodepool-sa"

	// Ensure resources exists
	ensureNamespace(&nodepoolEnv, nodepoolNamespace)
	utils.EnsureServiceAccount(&nodepoolEnv, sa)
	ensureRole(&nodepoolEnv, sa)
	token := ensureServiceAccountSecret(&nodepoolEnv, sa)
	nodepoolConfig := createKubeConfig(nodepoolContext, nodepoolNamespace, sa, token)
	bytes, err := clientcmd.Write(nodepoolConfig)
	if err != nil {
		panic(err)
	}

	if sfEnv.Ns == "-" {
		fmt.Printf("%s\n", bytes)
	} else {
		ensureKubeConfigSecret(sfEnv, bytes, sf.NodepoolProvidersSecretsName)
	}
}

var CreateNamespaceNodepoolCmd = &cobra.Command{
	Use:   "create-namespace-for-nodepool",
	Short: "Create the namespace for nodepool openshiftpods driver",
	Long:  "This command produce a KUBECONFIG file for nodepool",

	Run: func(cmd *cobra.Command, args []string) {
		nodepoolContext, _ := cmd.Flags().GetString("nodepool-context")
		nodepoolNamespace, _ := cmd.Flags().GetString("nodepool-namespace")
		sfContext, _ := cmd.Flags().GetString("sf-context")
		sfNamespace, _ := cmd.Flags().GetString("sf-namespace")
		sfEnv := utils.ENV{
			Cli: utils.CreateKubernetesClientOrDie(sfContext),
			Ctx: context.TODO(),
			Ns:  sfNamespace,
		}
		CreateNamespaceForNodepool(&sfEnv, nodepoolContext, nodepoolNamespace, sfContext)
	},
}

func init() {
	CreateNamespaceNodepoolCmd.Flags().StringP("nodepool-context", "", "", "The kubeconfig context for the nodepool-namespace, use the default context by default")
	CreateNamespaceNodepoolCmd.Flags().StringP("nodepool-namespace", "", "nodepool", "The namespace name for nodepool")
	CreateNamespaceNodepoolCmd.Flags().StringP("sf-context", "", "", "The kubeconfig context of the sf-namespace, use the default context by default")
	CreateNamespaceNodepoolCmd.Flags().StringP("sf-namespace", "", "sf", "Name of the namespace to copy the kubeconfig, or '-' for stdout")
}
