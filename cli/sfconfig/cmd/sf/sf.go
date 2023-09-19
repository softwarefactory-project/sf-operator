/*
Copyright © 2022-2023 Red Hat, Inc.
*/

// Package sf provides facilities for the sfconfig CLI
package sf

import (
	"os"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/sf/delete"
	"github.com/spf13/cobra"
)

// SfCmd represents the operator command
var SfCmd = &cobra.Command{
	Use:   "sf",
	Short: "Command related to Software Factory",
	Long:  `Command related to Software Factory`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(0)
	},
}

func init() {
	SfCmd.AddCommand(delete.DeleteCmd)
}
