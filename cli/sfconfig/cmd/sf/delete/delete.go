/*
Copyright © 2023 Red Hat
*/
package delete

import (
	"context"
	"fmt"
	"os"

	"github.com/apenella/go-ansible/pkg/options"
	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/spf13/cobra"
)

// operatorDeleteCmd represents the operatordelete command
var DeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Run playbook/wipe.yaml",
	Long: `Run playbook/wipe.yaml playbook, it can be used for CI job and can be used locally

./tools/sfconfig sf delete [OPTIONS]

OPTIONS
  --instance, -i - deletes Software Factory Instance
  --pvcs, -p - deletes Software Factory including PVCs and PVs
  --all, -a - executes --delete and --remove options in sequence
  --verbose, -v - verbose

	`,
	Run: func(cmd *cobra.Command, args []string) {
		instance, _ := cmd.Flags().GetBool("instance")
		pvcs, _ := cmd.Flags().GetBool("pvcs")
		all, _ := cmd.Flags().GetBool("all")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if !instance && !pvcs && !all {
			cmd.Help()
			os.Exit(0)
		}

		ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{}
		ansiblePlaybookConnectionOptions := &options.AnsibleConnectionOptions{}
		ansiblePlaybookConnectionOptions.Connection = "local"
		ansiblePlaybookOptions.AddExtraVar("remote_os_host", true)
		ansiblePlaybookOptions.AddExtraVar("hostname", "localhost")

		if verbose {
			ansiblePlaybookOptions.Verbose = true
		}

		if instance || all {
			ansiblePlaybookOptions.Tags += "sf_delete_instance,"
		}
		if pvcs || all {
			ansiblePlaybookOptions.Tags += "sf_delete_pcvs,"
		}

		var playbook_yaml string = "playbooks/wipe.yaml"

		playbook := &playbook.AnsiblePlaybookCmd{
			Playbooks:         []string{playbook_yaml},
			ConnectionOptions: ansiblePlaybookConnectionOptions,
			Options:           ansiblePlaybookOptions,
		}

		options.AnsibleForceColor()
		fmt.Println(playbook)
		err := playbook.Run(context.TODO())
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	DeleteCmd.Flags().BoolP("instance", "i", false, "deletes Software Factory Instance")
	DeleteCmd.Flags().BoolP("pvcs", "p", false, "deletes Software Factory including PVCs and PVs")
	DeleteCmd.Flags().BoolP("all", "a", false, "executes --delete and --remove options in sequence")
	DeleteCmd.Flags().BoolP("verbose", "v", false, "verbose")
}
