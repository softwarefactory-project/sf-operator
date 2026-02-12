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
	"bytes"
	"errors"
	"os"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	controllers "github.com/softwarefactory-project/sf-operator/controllers"

	"github.com/spf13/cobra"

	apiv1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

func restoreSecret(backupDir string, env *controllers.SFKubeContext, cr sfv1.SoftwareFactory) {
	ctrl.Log.Info("Restoring secrets...")

	for _, sec := range append(SecretsToBackup, controllers.CRSecrets(cr)...) {
		pathToSecret := backupDir + "/" + SecretsBackupPath + "/" + sec + ".yaml"
		data, err := os.ReadFile(pathToSecret)
		if err != nil {
			ctrl.Log.Error(err, "Couldn't read secret: "+pathToSecret)
			os.Exit(1)
		}
		var secret apiv1.Secret
		if err := yaml.Unmarshal(data, &secret); err != nil {
			ctrl.Log.Error(err, "Couldn't decode secret: "+pathToSecret)
			os.Exit(1)
		}
		secret.SetNamespace(env.Ns)
		env.CreateR(&secret)
	}

}

func restoreDB(backupDir string, env *controllers.SFKubeContext) {
	ctrl.Log.Info("Restoring DB...")
	pod := apiv1.Pod{}
	env.GetM(dbBackupPod, &pod)

	dropDBCMD := []string{
		"mysql",
		"-e DROP DATABASE zuul;",
	}
	env.PodExecM(pod.Name, controllers.MariaDBIdent, dropDBCMD)

	data, err := os.ReadFile(backupDir + "/" + DBBackupPath)
	if err != nil {
		ctrl.Log.Error(err, "Couldn't read sql dump")
		os.Exit(1)
	}

	err = env.PodExecIn("mariadb-0", "mariadb", []string{"mysql", "-h0"}, bytes.NewReader(data))
	if err != nil {
		ctrl.Log.Error(err, "Couldn't inject sql dump")
		os.Exit(1)
	}

	ctrl.Log.Info("Finished restoring DB from backup!")
}
func restoreZuul(backupDir string, env *controllers.SFKubeContext) {
	ctrl.Log.Info("Restoring Zuul...")

	// ensure that pod does not have any restore file
	cleanCMD := []string{
		"bash", "-c", "rm -rf /tmp/zuul-import && mkdir -p /tmp/zuul-import"}
	env.PodExecM("zuul-kazoo", "zuul-kazoo", cleanCMD)

	// copy the Zuul private keys backup to pod
	data, err := os.ReadFile(backupDir + "/" + ZuulBackupPath)
	if err != nil {
		ctrl.Log.Error(err, "Couldn't read zuul.keys")
		os.Exit(1)
	}

	// https://zuul-ci.org/docs/zuul/latest/client.html
	restoreCMD := []string{
		"bash", "-c", "cat > /tmp/zuul.keys && zuul-admin import-keys --force /tmp/zuul.keys"}

	// Execute command for restore
	err = env.PodExecIn("zuul-kazoo", "zuul-kazoo", restoreCMD, bytes.NewReader(data))
	if err != nil {
		ctrl.Log.Error(err, "Couldn't inject zuul.keys")
		os.Exit(1)
	}

	ctrl.Log.Info("Finished doing Zuul private keys restore!")

}

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

	restoreSecret(backupDir, env, cr)

	ctrl.Log.Info("Spawning backend services...")
	sfCtrl := controllers.MkSFController(*env, cr)
	sfCtrl.DeployMariadb()
	sfCtrl.DeployZookeeper()
	ctrl.Log.Info("Waiting for backend services...")
	controllers.WaitFor(sfCtrl.DeployMariadb)
	controllers.WaitFor(sfCtrl.DeployZookeeper)
	ctrl.Log.Info("Spawning zuul-kazoo...")
	sfCtrl.DeployZuulSecrets()
	sfCtrl.EnsureZuulConfigSecret(false)
	sfCtrl.EnsureKazooPod()
	controllers.WaitFor(sfCtrl.EnsureKazooPod)

	restoreZuul(backupDir, env)
	restoreDB(backupDir, env)

	sfCtrl.DeleteKazooPod()

	// Run deployment to ensure everything is running as expected.
	if err := env.StandaloneReconcile(cr); err != nil {
		ctrl.Log.Error(err, "Reconcille failed")
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
