// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

// Package gerrit provides gerrit utilities
package gerrit

import (
	"context"
	"fmt"
	"os"

	_ "embed"

	"github.com/spf13/cobra"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"

	cligerrit "github.com/softwarefactory-project/sf-operator/cli/cmd/dev/gerrit"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
)

var ns = "sf"

var GerritCmd = &cobra.Command{
	Use:   "gerrit",
	Short: "Deploy a demo Gerrit instance to hack on sf-operator",
	Run: func(cmd *cobra.Command, args []string) {
		deploy, _ := cmd.Flags().GetBool("deploy")
		wipe, _ := cmd.Flags().GetBool("wipe")
		fqdn, _ := cmd.Flags().GetString("fqdn")

		if !(deploy || wipe) {
			println("Select one of deploy or wipe option")
			os.Exit(1)
		}

		// Get the kube client
		cl := utils.CreateKubernetesClientOrDie("")
		ctx := context.Background()
		env := cliutils.ENV{
			Cli: cl,
			Ns:  ns,
			Ctx: ctx,
		}
		if deploy {
			fmt.Println("Ensure Gerrit deployed in namespace", ns)
			cligerrit.EnsureGerrit(&env, fqdn)
			fmt.Printf("Gerrit is available at https://gerrit.%s\n", fqdn)
		}

		if wipe {
			fmt.Println("Wipe Gerrit from namespace", ns)

			cligerrit.WipeGerrit(&env, false)
		}

	},
}

func init() {
	GerritCmd.Flags().BoolP("deploy", "", false, "Deploy Gerrit")
	GerritCmd.Flags().BoolP("wipe", "", false, "Wipe Gerrit deployment")
	GerritCmd.PersistentFlags().StringP("fqdn", "f", "sfop.me", "The FQDN of gerrit (gerrit.<FQDN>)")
}
