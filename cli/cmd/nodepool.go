// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

package cmd

/*
"nodepool" subcommands can be used to interact with and configure the Nodepool component of a SF deployment.
*/

import (
	"context"
	"errors"
	"os"

	apiv1 "k8s.io/api/core/v1"

	"github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

func get(kmd *cobra.Command, args []string) {
	cliCtx, err := GetCLIContext(kmd)
	if err != nil {
		ctrl.Log.Error(err, "Error initializing:")
		os.Exit(1)
	}
	argumentError := errors.New("argument must be in: providers-secrets, builder-ssh-key")
	if len(args) != 1 {
		ctrl.Log.Error(argumentError, "Need one argument")
		os.Exit(1)
	}
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
	} else if target == "builder-ssh-key" {
		pubKey, _ := kmd.Flags().GetString("pubkey")
		getBuilderSSHKey(ns, kubeContext, pubKey)
	} else {
		ctrl.Log.Error(argumentError, "Unknown argument "+target)
		os.Exit(1)
	}
}

func getProvidersSecret(ns string, kubeContext string, cloudsFile string, kubeFile string) {
	sfEnv := ENV{
		Cli: CreateKubernetesClientOrDie(kubeContext),
		Ctx: context.TODO(),
		Ns:  ns,
	}
	var secret apiv1.Secret
	if GetMOrDie(&sfEnv, controllers.NodepoolProvidersSecretsName, &secret) {
		if len(secret.Data["clouds.yaml"]) > 0 {
			if cloudsFile == "" {
				println("clouds.yaml:")
				println(string(secret.Data["clouds.yaml"]))
			} else {
				// TODO before we write to file, we should ensure the file, if it exists, is older than
				// the upstream secret to avoid losing more recent secrets.
				os.WriteFile(cloudsFile, secret.Data["clouds.yaml"], 0600)
				ctrl.Log.Info("File " + cloudsFile + " updated")
			}
		}
		if len(secret.Data["kube.config"]) > 0 {
			if kubeFile == "" {
				println("kube.config:")
				println(string(secret.Data["kube.config"]))
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
	sfEnv := ENV{
		Cli: CreateKubernetesClientOrDie(kubeContext),
		Ctx: context.TODO(),
		Ns:  ns,
	}
	var secret apiv1.Secret
	if GetMOrDie(&sfEnv, "nodepool-builder-ssh-key", &secret) {
		if pubKey == "" {
			println(string(secret.Data["pub"]))
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

func MkNodepoolCmd() *cobra.Command {

	var (
		cloudsOutput     string
		kubeconfigOutput string
		builderPubKey    string

		nodepoolCmd = &cobra.Command{
			Use:   "nodepool",
			Short: "Nodepool subcommands",
			Long:  `These subcommands can be used to interact with the Nodepool component of a Software Factory deployment.`,
		}
		createCmd, configureCmd, getCmd = GetCRUDSubcommands()
	)

	getCmd.Run = get
	getCmd.Use = "get {providers-secrets, builder-ssh-key}"
	getCmd.Long = "Get a Nodepool resource. The resource can be the providers secrets or the builder's public SSH key."
	getCmd.ValidArgs = []string{"providers-secrets", "builder-ssh-key"}
	getCmd.Flags().StringVar(&cloudsOutput, "clouds", "", "(use with providers-secrets) File where to dump the clouds secrets")
	getCmd.Flags().StringVar(&kubeconfigOutput, "kube", "", "(use with providers-secrets) File where to dump the kube secrets")
	getCmd.Flags().StringVar(&builderPubKey, "pubkey", "", "(use with builder-ssh-key) File where to dump nodepool-builder's SSH public key")

	createCmd.AddCommand(getCmd)

	nodepoolCmd.AddCommand(createCmd)
	nodepoolCmd.AddCommand(configureCmd)
	nodepoolCmd.AddCommand(getCmd)
	return nodepoolCmd
}
