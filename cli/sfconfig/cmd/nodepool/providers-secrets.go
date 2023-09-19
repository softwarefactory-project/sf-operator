/*
Copyright © 2023 Redhat
SPDX-License-Identifier: Apache-2.0
*/

// Package nodepool functions
package nodepool

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/config"
	"github.com/softwarefactory-project/sf-operator/controllers"
)

var ProvidersSecretsCmd = &cobra.Command{
	Use:   "nodepool-providers-secrets",
	Short: "Handle nodepool providers secrets",
	Long:  "This command provides capabilities to dump and update a clouds.yaml and a kube.config file for Nodepool",

	Run: func(cmd *cobra.Command, args []string) {
		sfContext, _ := cmd.Flags().GetString("sf-context")
		sfNamespace, _ := cmd.Flags().GetString("sf-namespace")
		updateOpt, _ := cmd.Flags().GetBool("update")
		dumpOpt, _ := cmd.Flags().GetBool("dump")
		sfEnv := utils.ENV{
			Cli: utils.CreateKubernetesClientOrDie(sfContext),
			Ctx: context.TODO(),
			Ns:  sfNamespace,
		}

		if updateOpt && dumpOpt || !updateOpt && !dumpOpt {
			cmd.Help()
			println()
			println("Either the 'update' or the 'dump' parameter must be set.")
			println()
			os.Exit(1)
		}

		conf := config.GetSFConfigOrDie()

		if updateOpt {
			clouds_content, _ := utils.GetFileContent(conf.Nodepool.CloudsFile)
			println(conf.Nodepool.CloudsFile)
			kube_content, _ := utils.GetFileContent(conf.Nodepool.KubeFile)
			ensureNodepoolProvidersSecrets(&sfEnv, clouds_content, kube_content)
		}

		if dumpOpt {
			var secret apiv1.Secret
			if utils.GetM(&sfEnv, controllers.NodepoolProvidersSecretsName, &secret) {
				if len(secret.Data["clouds.yaml"]) > 0 {
					os.WriteFile(conf.Nodepool.CloudsFile, secret.Data["clouds.yaml"], 0644)
					println("Updated " + conf.Nodepool.CloudsFile + " with secret content")
				}
				if len(secret.Data["kube.config"]) > 0 {
					os.WriteFile(conf.Nodepool.KubeFile, secret.Data["kube.config"], 0644)
					println("Updated " + conf.Nodepool.KubeFile + " with secret content")
				}
			} else {
				println("Unable to find Secret named: " + controllers.NodepoolProvidersSecretsName)
				os.Exit(1)
			}
		}
	},
}

func init() {
	ProvidersSecretsCmd.Flags().StringP("sf-context", "", "", "The kubeconfig context of the sf-namespace, use the default context by default")
	ProvidersSecretsCmd.Flags().StringP("sf-namespace", "", "sf", "Name of the namespace to copy the kubeconfig, or '-' for stdout")
	ProvidersSecretsCmd.Flags().BoolP("update", "u", false, "Update the providers secrets from local config to the sf namespace (exclusive with '-d')")
	ProvidersSecretsCmd.Flags().BoolP("dump", "d", false, "Dump the providers secrets from the sf namespace to the local config (exclusive with '-u')")
}
