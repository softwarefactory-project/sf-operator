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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	controllers "github.com/softwarefactory-project/sf-operator/controllers"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

func prepareRestore(kmd *cobra.Command) (string, *kubernetes.Clientset, string) {

	cliCtx, err := cliutils.GetCLIContext(kmd)
	if err != nil {
		ctrl.Log.Error(err, "Error initializing CLI:")
		os.Exit(1)
	}

	kubeContext := cliCtx.KubeContext
	_, kubeClientSet := cliutils.GetClientset(kubeContext)
	return cliCtx.Namespace, kubeClientSet, kubeContext
}

func restoreSecret(ns string, backupDir string, kubeContext string) {
	ctrl.Log.Info("Restoring secrets...")

	env := cliutils.ENV{
		Cli: cliutils.CreateKubernetesClientOrDie(kubeContext),
		Ctx: context.TODO(),
		Ns:  ns,
	}

	for _, sec := range SecretsToBackup {
		pathToSecret := backupDir + "/" + SecretsBackupPath + "/" + sec + ".yaml"
		secretContent := cliutils.ReadYAMLToMapOrDie(pathToSecret)

		var secret corev1.Secret
		if cliutils.GetMOrDie(&env, sec, &secret) {
			secretMap := secretContent["data"].(map[string]interface{})
			for key, value := range secretMap {
				stringValue, ok := value.(string)
				if !ok {
					ctrl.Log.Error(errors.New("can not convert secret data value to string"),
						"Can not restore secret"+sec)
					os.Exit(1)
				}
				secret.Data[key] = []byte(stringValue)
			}
		} else {
			ctrl.Log.Error(errors.New("the secret does not exist"),
				"The secret: "+sec+" should be available before continuing restore")
			os.Exit(1)
		}

		cliutils.UpdateROrDie(&env, &secret)
	}

}

func restoreDB(ns string, backupDir string, kubeClientSet *kubernetes.Clientset, kubeContext string) {
	ctrl.Log.Info("Restoring DB...")
	pod := cliutils.GetPodByName(dbBackupPod, ns, kubeClientSet)

	kubectlPath := cliutils.GetKubectlPath()
	dropDBCMD := []string{
		"mysql",
		"-e DROP DATABASE zuul;",
	}
	cliutils.RunRemoteCmd(kubeContext, ns, pod.Name, controllers.MariaDBIdent, dropDBCMD)

	mariadbBackupPath := backupDir + "/" + DBBackupPath

	// Below command is executing something like:
	//     cat backup/mariadb/db-zuul.sql | kubectl -n sf exec -it mariadb-0 -c mariadb -- sh -c "mysql -h0"
	// but in that case, we need to do it via system kubernetes client.
	executeCommand := fmt.Sprintf(
		"cat %s | %s -n %s exec -it %s -c %s -- sh -c \"mysql -h0\"",
		mariadbBackupPath, kubectlPath, ns, pod.Name, controllers.MariaDBIdent,
	)

	cliutils.ExecuteKubectlClient(ns, pod.Name, controllers.MariaDBIdent, executeCommand)

	ctrl.Log.Info("Finished restoring DB from backup!")
}
func restoreZuul(ns string, backupDir string, kubeClientSet *kubernetes.Clientset, kubeContext string) {
	ctrl.Log.Info("Restoring Zuul...")
	pod := cliutils.GetPodByName(zuulBackupPod, ns, kubeClientSet)

	// ensure that pod does not have any restore file
	restoreZuulRemoveCMD := []string{
		"rm",
		"-rf",
		"/tmp/zuul-import",
	}
	cliutils.RunRemoteCmd(kubeContext, ns, pod.Name, controllers.ZuulSchedulerIdent, restoreZuulRemoveCMD)

	// create empty directory for future restore
	restoreZuulCreateDirCMD := []string{
		"mkdir",
		"-p",
		"/tmp/zuul-import",
	}
	cliutils.RunRemoteCmd(kubeContext, ns, pod.Name, controllers.ZuulSchedulerIdent, restoreZuulCreateDirCMD)

	// copy the Zuul private keys backup to pod
	// tar cf - -C /tmp/backup/zuul zuul.keys | /usr/bin/kubectl exec -i -n sf zuul-scheduler-0 -c zuul-scheduler -- tar xf -  -C /tmp
	kubectlPath := cliutils.GetKubectlPath()
	basePath := filepath.Dir(backupDir + "/" + ZuulBackupPath)
	baseFile := filepath.Base(ZuulBackupPath)
	executeCommand := fmt.Sprintf(
		"tar cf - -C %s %s | %s exec -i -n %s %s -c %s -- tar xf - -C /tmp/zuul-import",
		basePath, baseFile, kubectlPath, ns, pod.Name, controllers.ZuulSchedulerIdent,
	)
	ctrl.Log.Info("Executing " + executeCommand)

	cliutils.ExecuteKubectlClient(ns, pod.Name, controllers.ZuulSchedulerIdent, executeCommand)

	// https://zuul-ci.org/docs/zuul/latest/client.html
	restoreZuulCMD := []string{
		"zuul-admin",
		"import-keys",
		"--force",
		"/tmp/zuul-import/" + baseFile,
	}

	// Execute command for restore
	cliutils.RunRemoteCmd(kubeContext, ns, pod.Name, controllers.ZuulSchedulerIdent, restoreZuulCMD)

	// remove after all
	cliutils.RunRemoteCmd(kubeContext, ns, pod.Name, controllers.ZuulSchedulerIdent, restoreZuulRemoveCMD)

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

	// prepare to make restore
	ns, kubeClientSet, kubeContext := prepareRestore(kmd)

	if ns == "" {
		ctrl.Log.Info("You did not specify the namespace!")
		os.Exit(1)
	}

	restoreZuul(ns, backupDir, kubeClientSet, kubeContext)
	restoreSecret(ns, backupDir, kubeContext)
	restoreDB(ns, backupDir, kubeClientSet, kubeContext)

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
