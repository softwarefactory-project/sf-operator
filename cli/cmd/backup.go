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
"backup" subcommand creates a backup of a deployment.
*/

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

func backupCmd(kmd *cobra.Command, args []string) {
	err := errors.New("backup is not supported yet")
	ctrl.Log.Error(err, "Command error")
	os.Exit(1)
}

func MkBackupCmd() *cobra.Command {

	var (
		backupCmd = &cobra.Command{
			Use:   "backup",
			Short: "Create a backup of a deployment",
			Long:  `This isn't implemented yet, this subcommand is a placeholder.`,
			Run:   backupCmd,
		}
	)

	return backupCmd
}
