// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

package cmd

/*
"nodepool" subcommands can be used to interact with and configure the Nodepool component of a SF deployment.
*/

import (
	"errors"
	"fmt"
	"os"

	apiv1 "k8s.io/api/core/v1"

	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

var npGetAllowedArgs = []string{"builder-ssh-key"}
var npCreateAllowedArgs = []string{"openshiftpods-namespace"}

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

		CreateNamespaceForNodepool(cliCtx, nodepoolContext, nodepoolNamespace)
		if showConfigTemplate {
			configTemplate := mkNodepoolOpenshiftPodsConfigTemplate(nodepoolNamespace)
			fmt.Println("Nodepool configuration template:")
			fmt.Println(configTemplate)
		}
	}
}

func CreateNamespaceForNodepool(sfEnv *controllers.SFKubeContext, nodepoolContext, nodepoolNamespace string) {
	npEnv, err := controllers.MkSFKubeContext("", nodepoolNamespace, nodepoolContext, false)
	if err != nil {
		logging.LogE(err, "Could not create nodepool kube client")
		os.Exit(1)
	}
	sfEnv.SetupK8SProvider(&npEnv)
}

func getBuilderSSHKey(sfEnv *controllers.SFKubeContext, pubKey string) {
	var secret apiv1.Secret
	if sfEnv.GetOrDie("nodepool-builder-ssh-key", &secret) {
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
				Context: controllers.OpenshiftPodsKubeContext,
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
