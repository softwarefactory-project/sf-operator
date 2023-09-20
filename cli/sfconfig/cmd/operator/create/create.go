/*
Copyright © 2023 Red Hat
*/

// Package create provides helper for creation
package create

import (
	"context"
	"fmt"
	"os"

	"github.com/apenella/go-ansible/pkg/options"
	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
	"github.com/spf13/cobra"
)

func getTemplate() string {
	template := `---
- hosts: "{{"{{ hostname }}"}}"
  {{ .Pretasks }}
  {{ .Tasks }}
`
	return template
}

func generateTemplate() string {
	type YamlData struct {
		Pretasks string
		Tasks    string
	}

	yamlData := &YamlData{
		Pretasks: `roles:
    - setup-namespaces`,
		Tasks: "",
	}

	template, err := utils.ParseString(getTemplate(), yamlData)
	if err != nil {
		fmt.Println(err)
		panic("Template parsing failed")
	}

	return template
}

// CreateCmd represents the operatordelete command
var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Run a generated at playbooks/sfconfig-operator-create-*",
	Long: `Run a generated at playbooks/sfconfig-operator-create-*, it can be used for CI job and can be used locally

./tools/sfconfig operator create [OPTIONS]

OPTIONS
    --bundle, -b - creates namespace for the bundle ( default: bundle-catalog-ns )
    --bundlenamespace string, -B string- creates namespace for the bundle with specific name
    --namespace, -n - creates namespace for Software Factory ( default: sf )
    --namespacename string, -N string - creates namespace for Software Factory with specific name
    --all, -a - executes all options in sequence
    --verbose, -v - verbose

	`,
	Run: func(cmd *cobra.Command, args []string) {
		bundle, _ := cmd.Flags().GetBool("bundle")
		bundlenamespace, _ := cmd.Flags().GetString("bundlenamespace")
		namespace, _ := cmd.Flags().GetBool("namespace")
		namespacename, _ := cmd.Flags().GetString("namespacename")
		all, _ := cmd.Flags().GetBool("all")
		verbose, _ := cmd.Flags().GetBool("verbose")

		fmt.Println(bundle)

		if !bundle && len(bundlenamespace) == 0 && !namespace && len(namespacename) == 0 && !all {
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

		if bundle || all {
			ansiblePlaybookOptions.Tags += "operator_bundle_namespace,"
		}

		if len(bundlenamespace) != 0 {
			ansiblePlaybookOptions.AddExtraVar("bundlenamspace", bundlenamespace)
		}

		if namespace || all {
			ansiblePlaybookOptions.Tags += "operator_sf_namespace,"
		}

		if len(namespacename) != 0 {
			ansiblePlaybookOptions.AddExtraVar("namespace", namespacename)
		}

		file, _ := utils.CreateTempPlaybookFile(generateTemplate())

		var playbookYAML = file.Name()

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

		// Delete temporay file
		utils.RemoveTempPlaybookFile(file)
	},
}

func init() {
	CreateCmd.Flags().BoolP("bundle", "b", false, "creates namespace for the bundle ( default: bundle-catalog-ns )")
	CreateCmd.Flags().StringP("bundlenamespace", "B", "", "creates namespace for the bundle with specific name")
	CreateCmd.Flags().BoolP("namespace", "n", false, "creates namespace for Software Factory ( default: sf )")
	CreateCmd.Flags().StringP("namespacename", "N", "", "creates namespace for Software Factory with specific name")
	CreateCmd.Flags().BoolP("all", "a", false, "executes all options in sequence")
	CreateCmd.Flags().BoolP("verbose", "v", false, "verbose")
}
