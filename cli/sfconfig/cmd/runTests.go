/*
Copyright © 2023 Red Hat
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/apenella/go-ansible/pkg/options"
	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/spf13/cobra"
)

var extravars []string

// runTestsCmd represents the runTests command
var runTestsCmd = &cobra.Command{
	Use:   "runTests",
	Short: "Run playbook/main.yaml",
	Long: `Run playbook/main.yaml playbook, it used for CI job
    and can be used locally

    The variables used are config/default/local_ci.yaml
    with --zuul, variables are config/default/zuul_ci.yaml
    Run test_only tag
    ./tools/sfconfig runTests --test-only
	`,
	Run: func(cmd *cobra.Command, args []string) {
		zuul, _ := cmd.Flags().GetBool("zuul")
		test_only, _ := cmd.Flags().GetBool("test-only")

		vars, _ := varListToMap(extravars)
		ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{}
		ansiblePlaybookConnectionOptions := &options.AnsibleConnectionOptions{}

		if zuul {
			ansiblePlaybookConnectionOptions.Connection = "local"
			ansiblePlaybookOptions.Inventory = "controller, "
			ansiblePlaybookOptions.ExtraVarsFile = []string{"@config/default/zuul_ci.yaml"}
		} else {
			ansiblePlaybookOptions.ExtraVarsFile = []string{"@config/default/local_ci.yaml"}
		}

		if test_only {
			ansiblePlaybookOptions.Tags = "test_only"
		}

		for keyVar, valueVar := range vars {
			ansiblePlaybookOptions.AddExtraVar(keyVar, valueVar)
		}

		playbook := &playbook.AnsiblePlaybookCmd{
			Playbooks:         []string{"playbooks/main.yaml"},
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

func varListToMap(varsList []string) (map[string]interface{}, error) {

	vars := map[string]interface{}{}

	for _, v := range varsList {
		tokens := strings.Split(v, "=")

		if len(tokens) != 2 {
			fmt.Println("extra-var needs to be defined as 'foo=bar'")
			os.Exit(1)
		}
		vars[tokens[0]] = tokens[1]
	}

	return vars, nil
}

func init() {
	rootCmd.AddCommand(runTestsCmd)
	runTestsCmd.Flags().StringSliceVarP(&extravars, "extra-var", "e", []string{}, "Set extra variables, the format of each variable must be <key>=<value>")
	runTestsCmd.Flags().BoolP("zuul", "l", false, "use config/default/zuul_ci.yaml")
	runTestsCmd.Flags().BoolP("test-only", "t", false, "run test_only")
}