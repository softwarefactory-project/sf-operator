/*
Copyright © 2023 Redhat
*/
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
           ./tools/sfconfig microshift --inventory ../inventory.yaml`,

	Run: func(cmd *cobra.Command, args []string) {
		skip_setup, _ := cmd.Flags().GetBool("skip-setup")
		skip_deploy, _ := cmd.Flags().GetBool("skip-deploy")

		ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{
			Inventory: inventory,
		}

		if !skip_setup {
			setup := &playbook.AnsiblePlaybookCmd{
				Playbooks: []string{"tools/microshift/local_setup.yaml"},
				Options:   ansiblePlaybookOptions,
			}
			fmt.Println(setup)
			err := setup.Run(context.TODO())
			if err != nil {
				panic(err)
			}
		}
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
	microshiftCmd.Flags().BoolP("skip-setup", "", false, "do not install local requirement")
	microshiftCmd.Flags().BoolP("skip-deploy", "", false, "do not deploy microshift")
}
