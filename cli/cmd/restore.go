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
	"fmt"
	"os"
	"path/filepath"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	controllers "github.com/softwarefactory-project/sf-operator/controllers"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func restoreSecret(backupDir string, env *controllers.SFKubeContext, cr sfv1.SoftwareFactory) {
	ctrl.Log.Info("Restoring secrets...")

	for _, sec := range append(SecretsToBackup, controllers.CRSecrets(cr)...) {
		pathToSecret := backupDir + "/" + SecretsBackupPath + "/" + sec + ".yaml"
		secretContent := cliutils.ReadYAMLToMapOrDie(pathToSecret)

		typeValue, _ := secretContent["type"].(apiv1.SecretType)
		secret := apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: sec, Namespace: env.Ns},
			Data:       map[string][]byte{}, Type: typeValue}
		secretMap := secretContent["data"].(map[string]any)
		for _, key := range maps.Keys(secretMap) {
			stringValue, ok := secretMap[key].(string)
			if !ok {
				ctrl.Log.Error(errors.New("can not convert secret data value to string"),
					"Can not restore secret "+sec)
				os.Exit(1)
			}
			secret.Data[key] = []byte(stringValue)
		}
		env.CreateR(&secret)
	}

}

func restoreDB(backupDir string, env *controllers.SFKubeContext) {
	ctrl.Log.Info("Restoring DB...")
	pod := apiv1.Pod{}
	env.GetM(dbBackupPod, &pod)

	kubectlPath := cliutils.GetKubectlPath()
	dropDBCMD := []string{
		"mysql",
		"-e DROP DATABASE zuul;",
	}
	env.PodExecM(pod.Name, controllers.MariaDBIdent, dropDBCMD)

	mariadbBackupPath := backupDir + "/" + DBBackupPath

	// Below command is executing something like:
	//     cat backup/mariadb/db-zuul.sql | kubectl -n sf exec -it mariadb-0 -c mariadb -- sh -c "mysql -h0"
	// but in that case, we need to do it via system kubernetes client.
	executeCommand := fmt.Sprintf(
		"cat %s | %s -n %s exec -it %s -c %s -- sh -c \"mysql -h0\"",
		mariadbBackupPath, kubectlPath, env.Ns, pod.Name, controllers.MariaDBIdent,
	)

	cliutils.ExecuteKubectlClient(env.Ns, pod.Name, controllers.MariaDBIdent, executeCommand)

	ctrl.Log.Info("Finished restoring DB from backup!")
}
func restoreZuul(backupDir string, env *controllers.SFKubeContext) {
	ctrl.Log.Info("Restoring Zuul...")
	pod := apiv1.Pod{}
	env.GetM(zuulBackupPod, &pod)

	// ensure that pod does not have any restore file
	cleanCMD := []string{
		"bash", "-c", "rm -rf /tmp/zuul-import && mkdir -p /tmp/zuul-import"}
	env.PodExecM(pod.Name, controllers.ZuulSchedulerIdent, cleanCMD)

	// copy the Zuul private keys backup to pod
	// tar cf - -C /tmp/backup/zuul zuul.keys | /usr/bin/kubectl exec -i -n sf zuul-scheduler-0 -c zuul-scheduler -- tar xf -  -C /tmp
	kubectlPath := cliutils.GetKubectlPath()
	basePath := filepath.Dir(backupDir + "/" + ZuulBackupPath)
	baseFile := filepath.Base(ZuulBackupPath)
	executeCommand := fmt.Sprintf(
		"tar cf - -C %s %s | %s exec -i -n %s %s -c %s -- tar xf - -C /tmp/zuul-import",
		basePath, baseFile, kubectlPath, env.Ns, pod.Name, controllers.ZuulSchedulerIdent,
	)
	ctrl.Log.Info("Executing " + executeCommand)

	cliutils.ExecuteKubectlClient(env.Ns, pod.Name, controllers.ZuulSchedulerIdent, executeCommand)

	// https://zuul-ci.org/docs/zuul/latest/client.html
	restoreCMD := []string{
		"bash", "-c", "zuul-admin import-keys --force /tmp/zuul-import/" + baseFile + " && " +
			"rm -rf /tmp/zuul-import"}

	// Execute command for restore
	env.PodExecM(pod.Name, controllers.ZuulSchedulerIdent, restoreCMD)

	ctrl.Log.Info("Finished doing Zuul private keys restore!")

}

func clearComponents(env *controllers.SFKubeContext) {
	ctrl.Log.Info("Removing components requiring a complete restart ...")

	// --- Scale down Zuul STS components
	components := []string{"zuul-executor", "zuul-merger", "zuul-scheduler"}
	ctrl.Log.Info("Scaling down StatefulSets before deletion...")
	for _, comp := range components {
		if err := env.ScaleDownSTSAndWait(comp); err != nil {
			ctrl.Log.Error(err, "Could not scale down StatefulSet, continuing...", "name", comp)
		}
	}

	// --- Stop all STS now
	for _, stsName := range []string{"zuul-executor", "zuul-merger", "zuul-scheduler"} {
		env.DeleteOrDie(&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      stsName,
				Namespace: env.Ns,
			},
		})
	}

	// --- Stop deployments
	for _, depName := range []string{"zuul-web", "zuul-weeder", "nodepool-launcher"} {
		env.DeleteOrDie(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      depName,
				Namespace: env.Ns,
			},
		})
	}
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

	if err := env.StandaloneReconcile(cr, true); err != nil {
		ctrl.Log.Error(err, "Deployment failed")
	}

	restoreZuul(backupDir, env)
	restoreDB(backupDir, env)
	clearComponents(env)

	// Re-run deployment to ensure everything is running as expected.
	if err := env.StandaloneReconcile(cr, false); err != nil {
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
