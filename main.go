// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/softwarefactory-project/sf-operator/cli/cmd"
	dev "github.com/softwarefactory-project/sf-operator/cli/cmd/dev"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	zuul "github.com/softwarefactory-project/sf-operator/cli/cmd/zuul"
	"github.com/softwarefactory-project/sf-operator/controllers"
)

var dryRun bool

func deployCmd(kmd *cobra.Command, args []string) {
	cliutils.SetLogger(kmd)

	ns, _ := kmd.Flags().GetString("namespace")
	kubeContext, _ := kmd.Flags().GetString("kube-context")

	remotePath, _ := kmd.Flags().GetString("remote")

	crPath := args[0]

	if crPath == "" {
		fmt.Printf("Missing CR to deploy!\n")
		fmt.Printf("usage: deploy <path-to-cr>\n")
		os.Exit(1)
	}
	controllers.Standalone(ns, kubeContext, dryRun, crPath, remotePath)
}

func rotateCmd(kmd *cobra.Command, args []string) {
	cliutils.SetLogger(kmd)

	ns, _ := kmd.Flags().GetString("namespace")
	kubeContext, _ := kmd.Flags().GetString("kube-context")

	crPath := args[0]

	if crPath == "" {
		fmt.Printf("Missing CR to deploy!\n")
		fmt.Printf("usage: rotate-secrest <path-to-cr>\n")
		os.Exit(1)
	}
	if err := controllers.RotateSecrets(ns, kubeContext, dryRun, crPath); err != nil {
		fmt.Printf("Rotation failed: %s\n", err)
		os.Exit(1)
	}
}

func main() {

	var (
		ns          string
		kubeContext string
		fqdn        string

		rootCmd = &cobra.Command{Short: "SF Operator CLI",
			Long: `Multi-purpose command line utility related to sf-operator, SF instances management, and development tools.`,
		}

		deployCmd = &cobra.Command{
			Use:   "deploy [The path to the CR defining the Software Factory deployment.]",
			Short: "Start SF Operator as standalone",
			Long:  `This command starts a sf-operator deployment locally, without the need to install or run the software factory operator controller`,
			Run:   deployCmd,
		}

		rotateCmd = &cobra.Command{
			Use:   "rotate-secrets [The path to the CR defining the Software Factory deployment.]",
			Short: "Perform secret rotations",
			Long:  `This command rotates the internal secret used by the services`,
			Run:   rotateCmd,
		}
	)

	// Flags for the deploy command
	deployCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Shows what resources will be changed by a deploy operation")
	var remote string
	deployCmd.PersistentFlags().StringVarP(&remote, "remote", "r", "", "Remote CR")

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&ns, "namespace", "n", "", "The namespace on which to perform actions.")
	rootCmd.PersistentFlags().StringVarP(&kubeContext, "kube-context", "k", "", "The cluster context to use to perform calls to the K8s API.")
	rootCmd.PersistentFlags().StringVarP(&fqdn, "fqdn", "d", "", "The FQDN of the deployment (if no manifest is provided).")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable DEBUG logs")

	// Add sub commands
	subcommands := []*cobra.Command{
		cmd.MkInitCmd(),
		cmd.MkSFCmd(),
		cmd.MkNodepoolCmd(),
		cmd.MkVersionCmd(),
		dev.MkDevCmd(),
		zuul.MkZuulCmd(),
		deployCmd,
		rotateCmd,
	}
	for _, c := range subcommands {
		rootCmd.AddCommand(c)
	}
	rootCmd.Execute()
}
