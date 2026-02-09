/*
Copyright © 2023-2024 Red Hat

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

	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

func backupCmd(kmd *cobra.Command, args []string) {
	backupDir, _ := kmd.Flags().GetString("backup_dir")

	if backupDir == "" {
		ctrl.Log.Error(errors.New("no backup dir set"), "You need to set --backup_dir parameter!")
		os.Exit(1)
	}

	env, cr := cliutils.GetCLICRContext(kmd, args)

	if env.Ns == "" {
		ctrl.Log.Error(errors.New("no namespace set"), "You need to specify the namespace!")
		os.Exit(1)
	}

	if env.Owner.GetName() == "" {
		ctrl.Log.Error(errors.New("no owner found"), "Software Factory doesn't seem to be running?!")
		os.Exit(1)
	}

	// TODO: check that the CR name and the FQDN match the cr being backuped
	if err := env.DoBackup(backupDir, cr); err != nil {
		os.Exit(1)
	}
}

func MkBackupCmd() *cobra.Command {

	var (
		backupDir string
		backupCmd = &cobra.Command{
			Use:   "backup",
			Short: "Create a backup of a deployment",
			Long:  `This command will do a backup of important resources`,
			Run:   backupCmd,
		}
	)

	backupCmd.Flags().StringVar(&backupDir, "backup_dir", "", "The path to the backup directory")
	return backupCmd
}
