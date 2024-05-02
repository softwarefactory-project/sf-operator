/*
Copyright © 2023 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

/*
"sf" subcommand relates to managing a Software Factory resource.
*/

import (
	bootstraptenantconfigrepo "github.com/softwarefactory-project/sf-operator/cli/cmd/bootstrap-tenant-config-repo"
	"github.com/spf13/cobra"
)

func MkSFCmd() *cobra.Command {

	var sfCmd = &cobra.Command{
		Use:   "SF",
		Short: "subcommands related to managing a Software Factory resource",
		Long:  `Use these subcommands to perform management tasks at the resource level.`,
	}

	sfCmd.AddCommand(MkBackupCmd())
	sfCmd.AddCommand(MkRestoreCmd())
	sfCmd.AddCommand(bootstraptenantconfigrepo.MkBootstrapCmd())

	return sfCmd
}
