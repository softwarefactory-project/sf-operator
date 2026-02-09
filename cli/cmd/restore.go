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
"restore" subcommand restores a deployment to an existing backup.
*/

import (
	"errors"
	"os"

	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"

	"github.com/spf13/cobra"

	ctrl "sigs.k8s.io/controller-runtime"
)

func restoreCmd(kmd *cobra.Command, args []string) {

	// NOTE: Solution for restoring DB and Zuul require kubectl binary to be installed and configured .kube/config
	// file as well.
	// With that way, we don't need to copy the restore file/dir to the pod or create new pod with new PV or
	// mount same PVC into new pod, which might be rejected by some PV drivers. Also mounting local host directory
	// to the OpenShift cluster might be prohibited in some deployments (especially in public deployments where
	// user is not an admin), so that is not a good idea to use.

	backupDir, _ := kmd.Flags().GetString("backup_dir")

	if backupDir == "" {
		ctrl.Log.Error(errors.New("not enough parameters"),
			"The '--backup-dir' parameter needs to be set")
		os.Exit(1)

	}

	env, cr := cliutils.GetCLICRContext(kmd, args)

	if env.Ns == "" {
		ctrl.Log.Info("You did not specify the namespace!")
		os.Exit(1)
	}

	if env.Owner.GetName() != "" {
		ctrl.Log.Error(errors.New("sf owner exist"), "Software Factory should not be running")
		os.Exit(1)
	}

	env.EnsureStandaloneOwner(cr.Spec)

	if err := env.DoRestore(backupDir, cr); err != nil {
		os.Exit(1)
	}
}

func MkRestoreCmd() *cobra.Command {

	var (
		backupDir  string
		restoreCmd = &cobra.Command{
			Use:   "restore",
			Short: "Restore a deployment to a previous backup",
			Run:   restoreCmd,
		}
	)
	restoreCmd.Flags().StringVar(&backupDir, "backup_dir", "", "The path to the dir where backup is located")

	return restoreCmd
}
