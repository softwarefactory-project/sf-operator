/*
Copyright © 2023 Redhat
SPDX-License-Identifier: Apache-2.0
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/clientcmd"
	cliapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
)

type ENV struct {
	cli client.Client
	ns  string
	ctx context.Context
}

// Helper to fetch a kubernetes resource by name, returns true when it is found.
func (e *ENV) getM(name string, obj client.Object) bool {
	err := e.cli.Get(e.ctx,
		client.ObjectKey{
			Name:      name,
			Namespace: e.ns,
		}, obj)
	if errors.IsNotFound(err) {
		return false
	} else if err != nil {
		panic(fmt.Errorf("Could not get %s: %s", name, err))
	}
	return true
}

// Helper to create a kubernetes resource.
func (e *ENV) createR(obj client.Object) {
	fmt.Fprintf(os.Stderr, "Creating %s in %s\n", obj.GetName(), e.ns)
	obj.SetNamespace(e.ns)
	if err := e.cli.Create(e.ctx, obj); err != nil {
		panic(fmt.Errorf("Could not create %s: %s", obj, err))
	}
}

func (e *ENV) ensureNamespace(name string) {
	var ns apiv1.Namespace
	if err := e.cli.Get(e.ctx, client.ObjectKey{Name: name}, &ns); errors.IsNotFound(err) {
		ns.Name = name
		e.createR(&ns)
	} else if err != nil {
		panic(fmt.Errorf("Could not get namespace: %s", err))
	}
}

func (e *ENV) ensureServiceAccount(name string) {
	var sa apiv1.ServiceAccount
	if !e.getM(name, &sa) {
		sa.Name = name
		e.createR(&sa)
	}
}

func (e *ENV) ensureRole(sa string) {
	var role rbacv1.Role
	if !e.getM("nodepool-role", &role) {
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
		e.createR(&role)
	} else {
		// TODO: update if needed
	}

	var rb rbacv1.RoleBinding
	if !e.getM("nodepool-rb", &rb) {
		rb.Name = "nodepool-rb"
		rb.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: sa}}
		rb.RoleRef.Kind = "Role"
		rb.RoleRef.Name = "nodepool-role"
		rb.RoleRef.APIGroup = "rbac.authorization.k8s.io"
		e.createR(&rb)
	}
}

func (e *ENV) ensureServiceAccountSecret(sa string) string {
	var secret apiv1.Secret
	if !e.getM("nodepool-token", &secret) {
		secret.Name = "nodepool-token"
		secret.ObjectMeta.Annotations = map[string]string{
			"kubernetes.io/service-account.name": sa,
		}
		secret.Type = "kubernetes.io/service-account-token"
		e.createR(&secret)
	}
	for retry := 1; retry < 20; retry++ {
		token := secret.Data["token"]
		if token != nil {
			return string(token)
		}
		time.Sleep(time.Second)
		e.getM("nodepool-token", &secret)
	}
	panic("Could not find token")
}

func createKubeConfig(ns string, sa string, token string) cliapi.Config {
	currentConfig := config.GetConfigOrDie()
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

func (e *ENV) ensureKubeConfigSecret(config []byte, name string) {
	var secret apiv1.Secret
	if !e.getM(name, &secret) {
		secret.Name = name
		secret.Data = map[string][]byte{"kube.config": config}
		e.createR(&secret)
	} else {
		// TODO: update data if needed
	}
}

func CreateNamespaceForNodepool(ns string, output string) {
	ctx := context.TODO()
	cli := utils.CreateKubernetesClient()
	sa := "nodepool-sa"
	e := ENV{cli: cli, ctx: ctx, ns: ns}

	// Ensure resources exists
	e.ensureNamespace(ns)
	e.ensureServiceAccount(sa)
	e.ensureRole(sa)
	token := e.ensureServiceAccountSecret(sa)
	nodepoolConfig := createKubeConfig(ns, sa, token)
	bytes, err := clientcmd.Write(nodepoolConfig)
	if err != nil {
		panic(err)
	}

	e.ns = output
	if e.ns == "-" {
		fmt.Printf("%s\n", bytes)
	} else {
		e.ensureKubeConfigSecret(bytes, "nodepool-providers-secrets")
	}
}

var createNamespaceNodepoolCmd = &cobra.Command{
	Use:   "create-namespace-for-nodepool",
	Short: "Create the namespace for nodepool openshiftpods driver",
	Long:  "This command produce a KUBECONFIG file for nodepool",

	Run: func(cmd *cobra.Command, args []string) {
		ns, _ := cmd.Flags().GetString("namespace")
		output, _ := cmd.Flags().GetString("output")
		CreateNamespaceForNodepool(ns, output)
	},
}

func init() {
	rootCmd.AddCommand(createNamespaceNodepoolCmd)
	createNamespaceNodepoolCmd.Flags().StringP("namespace", "", "nodepool", "Specify ansible playbook inventory")
	createNamespaceNodepoolCmd.Flags().StringP("output", "", "sf", "Name of the namespace to copy the kubeconfig, or '-' for stdout")
}
