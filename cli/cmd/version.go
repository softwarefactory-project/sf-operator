// Copyright (C) 2025 Red Hat
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	"github.com/spf13/cobra"
)

func MkVersionCmd() *cobra.Command {
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of sf-operator",
		Long:  `All software has versions. This is sf-operator's`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(utils.GetVersion())
		},
	}
	return versionCmd
}
