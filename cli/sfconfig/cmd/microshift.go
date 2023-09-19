/*
Copyright © 2023 Redhat
*/

// Package cmd provides cmd utilities
package cmd

import (
	"context"
	"fmt"

	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/spf13/cobra"
)

var inventory string

// microshiftCmd represents the microshift command
var microshiftCmd = &cobra.Command{
	Use:   "microshift",
	Short: "subcommand to install and validate microshift deployment",
	Long: `subcommand to install/validate microshift
           ./tools/sfconfig microshift --inventory ./tools/microshift/my-inventory.yaml`,

	Run: func(cmd *cobra.Command, args []string) {
		skip_local_setup, _ := cmd.Flags().GetBool("skip-local-setup")
		skip_deploy, _ := cmd.Flags().GetBool("skip-deploy")

		ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{
			Inventory: inventory,
		}

		// Here we ensure we have the ansible-microshift-role available
		microshift_role_setup := &playbook.AnsiblePlaybookCmd{
			Playbooks: []string{"tools/microshift/ansible-microshift-role.yaml"},
			Options:   ansiblePlaybookOptions,
		}
		fmt.Println(microshift_role_setup)
		err := microshift_role_setup.Run(context.TODO())
		if err != nil {
			panic(err)
		}

		// Here we setup the local environment (packages deps, local resolver, ...)
		if !skip_local_setup {
			local_setup := &playbook.AnsiblePlaybookCmd{
				Playbooks: []string{"tools/microshift/local-setup.yaml"},
				Options:   ansiblePlaybookOptions,
			}
			fmt.Println(local_setup)
			err := local_setup.Run(context.TODO())
			if err != nil {
				panic(err)
			}
		}

		// Here we setup the remote microshift machine and we fetch a working kube/config
		if !skip_deploy {
			ansiblePlaybookOptions.ExtraVarsFile = []string{"@tools/microshift/group_vars/all.yaml"}
			deploy := &playbook.AnsiblePlaybookCmd{
				Playbooks: []string{"tools/microshift/deploy-microshift.yaml"},
				Options:   ansiblePlaybookOptions,
			}
			fmt.Println(deploy)
			err := deploy.Run(context.TODO())
			if err != nil {
				panic(err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(microshiftCmd)
	microshiftCmd.Flags().StringVarP(&inventory, "inventory", "i", "", "Specify ansible playbook inventory")
	microshiftCmd.Flags().BoolP("skip-local-setup", "", false, "do not install local requirements")
	microshiftCmd.Flags().BoolP("skip-deploy", "", false, "do not deploy microshift")
}
