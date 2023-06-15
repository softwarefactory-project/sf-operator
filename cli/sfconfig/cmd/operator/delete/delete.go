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

./tools/sfconfig operator delete [OPTIONS]

OPTIONS
  --subscription, -s - deletes Software Factory Operator's Subscription
  --catalogsource, -S - deletes Software Factory Catalog Source
  --clusterserviceversion, -c - deletes Software Factory Cluster Service Version
  --all, -a - executes all options in sequence
  --verbose, -v - verbose

	`,
	Run: func(cmd *cobra.Command, args []string) {
		subscription, _ := cmd.Flags().GetBool("subscription")
		catalogsource, _ := cmd.Flags().GetBool("catalogsource")
		clusterserviceversion, _ := cmd.Flags().GetBool("clusterserviceversion")
		all, _ := cmd.Flags().GetBool("all")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if !subscription && !catalogsource && !clusterserviceversion && !all {
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

		if subscription || all {
			ansiblePlaybookOptions.Tags += "op_delete_sub,"
		}
		if catalogsource || all {
			ansiblePlaybookOptions.Tags += "op_delete_catsrc,"
		}
		if clusterserviceversion || all {
			ansiblePlaybookOptions.Tags += "op_delete_csv,"
		}

		var playbook_yaml string = "playbooks/wipe.yaml"

		playbook := &playbook.AnsiblePlaybookCmd{
			Playbooks:         []string{playbook_yaml},
			ConnectionOptions: ansiblePlaybookConnectionOptions,
			Options:           ansiblePlaybookOptions,
		}
		// TODO add option to get color locally
		// options.AnsibleForceColor()
		fmt.Println(playbook)
		err := playbook.Run(context.TODO())
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	DeleteCmd.Flags().BoolP("subscription", "s", false, "deletes Software Factory Operator's Subscription")
	DeleteCmd.Flags().BoolP("catalogsource", "S", false, "deletes Software Factory Catalog Source")
	DeleteCmd.Flags().BoolP("clusterserviceversion", "c", false, "deletes Software Factory Cluster Service Version")
	DeleteCmd.Flags().BoolP("all", "a", false, "executes all options in sequence")
	DeleteCmd.Flags().BoolP("verbose", "v", false, "verbose")
}
