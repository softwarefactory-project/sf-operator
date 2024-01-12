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
		skipLocalSetup, _ := cmd.Flags().GetBool("skip-local-setup")
		skipDeploy, _ := cmd.Flags().GetBool("skip-deploy")
		skipPostInstall, _ := cmd.Flags().GetBool("skip-post-install")

		ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{
			Inventory: inventory,
		}

		var err error

		// Here we ensure we have the ansible-microshift-role available
		microshiftRoleSetup := &playbook.AnsiblePlaybookCmd{
			Playbooks: []string{"tools/microshift/ansible-microshift-role.yaml"},
			Options:   ansiblePlaybookOptions,
		}
		fmt.Println(microshiftRoleSetup)
		err = microshiftRoleSetup.Run(context.TODO())
		if err != nil {
			panic(err)
		}

		// Here we setup the local environment (packages deps, local resolver, ...)
		if !skipLocalSetup {
			localSetup := &playbook.AnsiblePlaybookCmd{
				Playbooks: []string{"tools/microshift/local-setup.yaml"},
				Options:   ansiblePlaybookOptions,
			}
			fmt.Println(localSetup)
			err = localSetup.Run(context.TODO())
			if err != nil {
				panic(err)
			}
		}

		// Here we setup the remote microshift machine and we fetch a working kube/config
		if !skipDeploy {

			ansiblePlaybookOptions.ExtraVarsFile = []string{"@tools/microshift/group_vars/all.yaml"}
			deploy := &playbook.AnsiblePlaybookCmd{
				Playbooks: []string{"tools/microshift/deploy-microshift.yaml"},
				Options:   ansiblePlaybookOptions,
			}
			fmt.Println(deploy)
			err = deploy.Run(context.TODO())
			if err != nil {
				panic(err)
			}
		}

		// Prepare namespaces and install required operators
		if !skipPostInstall {
			ansiblePlaybookOptions.ExtraVarsFile = []string{"@tools/microshift/group_vars/all.yaml"}
			postinstall := &playbook.AnsiblePlaybookCmd{
				Playbooks: []string{"tools/microshift/post-install.yaml"},
				Options:   ansiblePlaybookOptions,
			}
			fmt.Println(postinstall)
			err = postinstall.Run(context.TODO())
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
	microshiftCmd.Flags().BoolP("skip-post-install", "", false, "do not setup namespaces and install operator dependencies")
}
