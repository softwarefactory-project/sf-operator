/*
Copyright © 2023 Red Hat
*/

// Package cmd provides facilities to run the functional test suite
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
	Short: "Run the test suite",
	Run: func(cmd *cobra.Command, args []string) {
		testOnly, _ := cmd.Flags().GetBool("standalone")
		upgrade, _ := cmd.Flags().GetBool("upgrade")
		verbose, _ := cmd.Flags().GetBool("v")
		debug, _ := cmd.Flags().GetBool("vvv")

		vars, _ := varListToMap(extravars)
		ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{}
		ansiblePlaybookConnectionOptions := &options.AnsibleConnectionOptions{}

		ansiblePlaybookOptions.AddExtraVar("hostname", "localhost")

		if verbose {
			ansiblePlaybookOptions.VerboseV = true
		}

		if debug {
			ansiblePlaybookOptions.VerboseVVVV = true
		}

		for keyVar, valueVar := range vars {
			ansiblePlaybookOptions.AddExtraVar(keyVar, valueVar)
		}

		var playbookYAML string
		if upgrade {
			playbookYAML = "playbooks/upgrade.yaml"
		} else {
			playbookYAML = "playbooks/main.yaml"
			if testOnly {
				ansiblePlaybookOptions.Tags = "standalone"
				ansiblePlaybookOptions.AddExtraVar("mode", "standalone")
			} else {
				ansiblePlaybookOptions.AddExtraVar("mode", "olm")
			}
		}

		playbook := &playbook.AnsiblePlaybookCmd{
			Playbooks:         []string{playbookYAML},
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
	runTestsCmd.Flags().BoolP("standalone", "s", false, "run standalone test")
	runTestsCmd.Flags().BoolP("upgrade", "u", false, "run upgrade test")
	runTestsCmd.Flags().Bool("v", false, "run ansible in verbose mode")
	runTestsCmd.Flags().Bool("vvv", false, "run ansible in debug mode")
}
