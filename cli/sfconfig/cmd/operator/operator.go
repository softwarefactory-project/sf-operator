/*
Copyright © 2022-2023 Red Hat, Inc.
*/

// Package operator provides operator functions
package operator

import (
	"os"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/operator/create"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/operator/delete"
	"github.com/spf13/cobra"
)

// OperatorCmd represents the operator command
var OperatorCmd = &cobra.Command{
	Use:   "operator",
	Short: "Command related to Software Factory Operator",
	Long:  `Command related to Software Factory Operator`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(0)
	},
}

func init() {
	OperatorCmd.AddCommand(delete.DeleteCmd)
	OperatorCmd.AddCommand(create.CreateCmd)
}
