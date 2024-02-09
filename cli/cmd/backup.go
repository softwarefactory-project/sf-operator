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
	controllers "github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	zuulBackupPod = "zuul-scheduler-0"
	dbBackupPod   = "mariadb-0"
)

// Short legend what to backup
// - ca-cert - this is the local CA root certificate material. We might need keep it because it is used to
//             generate the zookeeper-client-tls and zookeeper-server-tls secrets.
//             The zookeeper-client-tls will be used by external zuul component like the executor
// - zookeeper-client-tls
// - zookeeper-server-tls
// - nodepool-builder-ssh-key - this key pair is used to connect on an image-builder machine. The builder machine
//                              have the pub key part in the .ssh/authorized_keys file
// - zuul-ssh-key This is the key pair used by Zuul to connect on external system - like gerrit.
//                This key is added as authorized keys on external system
// - zuul-keystore-password - this is the key used to encrypt/decrypt key pairs stored into zookeeper
// - zuul-auth-secret - this contains the secret for the zuul-client connection
// - mariadb-root-password - this contains MariaDB root password

var secretsToBackup = []string{
	"ca-cert",
	"zookeeper-client-tls",
	"zookeeper-server-tls",
	"nodepool-builder-ssh-key",
	"zuul-ssh-key",
	"zuul-keystore-password",
	"zuul-auth-secret",
	"mariadb-root-password",
}

func prepareBackup(kmd *cobra.Command, backupDir string) (string, *kubernetes.Clientset, string) {

	cliCtx, err := cliutils.GetCLIContext(kmd)
	if err != nil {
		ctrl.Log.Error(err, "Error initializing CLI:")
		os.Exit(1)
	}

	cliutils.CreateDirectory(backupDir, 0755)

	kubeContext := cliCtx.KubeContext
	_, kubeClientSet := cliutils.GetClientset(kubeContext)
	return cliCtx.Namespace, kubeClientSet, kubeContext
}

func createSecretBackup(ns string, backupDir string, kubeClientSet *kubernetes.Clientset) {
	ctrl.Log.Info("Creating secrets backup...")

	secretsDir := backupDir + "/secrets"
	cliutils.CreateDirectory(secretsDir, 0755)

	for _, sec := range secretsToBackup {
		secret := cliutils.GetSecretByName(sec, ns, kubeClientSet)

		// convert secret content to string (was bytes)
		strMap := cliutils.ConvertMapOfBytesToMapOfStrings(secret.Data)

		// create new map with important content
		dataMap := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]string{
				"name": secret.Name,
			},
			"type": secret.Type,
			"data": strMap,
		}

		// dump to yaml
		yamlData, err := yaml.Marshal(dataMap)
		if err != nil {
			ctrl.Log.Error(err, "Can not dump to yaml")
			os.Exit(1)
		}

		// write to file
		cliutils.WriteContentToFile(secretsDir+"/"+secret.Name+".yaml", yamlData, 0640)
	}
	ctrl.Log.Info("Finished doing secret backup!")
}

func createZuulKeypairBackup(ns string, backupDir string, kubeClientSet *kubernetes.Clientset,
	kubeContext string) {

	ctrl.Log.Info("Doing Zuul keys backup...")

	pod := cliutils.GetPodByName(zuulBackupPod, ns, kubeClientSet)

	zuulBackupDir := backupDir + "/zuul/"
	cliutils.CreateDirectory(zuulBackupDir, 0755)
	backupZuulCMD := []string{
		"zuul",
		"export-keys",
		"/tmp/zuul-backup",
	}
	backupZuulPrintCMD := []string{
		"cat",
		"/tmp/zuul-backup",
	}
	backupZuulRemoveCMD := []string{
		"rm",
		"/tmp/zuul-backup",
	}

	// Execute command for backup
	cliutils.RunRemoteCmd(kubeContext, ns, pod.Name, controllers.ZuulSchedulerIdent, backupZuulCMD)

	// Take output of the backup
	commandBuffer := cliutils.RunRemoteCmd(kubeContext, ns, pod.Name, controllers.ZuulSchedulerIdent, backupZuulPrintCMD)

	// write stdout to file
	cliutils.WriteContentToFile(zuulBackupDir+"zuul.keys", commandBuffer.Bytes(), 0640)

	// Remove key file from the pod
	cliutils.RunRemoteCmd(kubeContext, ns, pod.Name, controllers.ZuulSchedulerIdent, backupZuulRemoveCMD)

	ctrl.Log.Info("Finished doing Zuul private keys backup!")
}

func createMySQLBackup(ns string, backupDir string, kubeClientSet *kubernetes.Clientset,
	kubeContext string) {
	ctrl.Log.Info("Doing DB backup...")

	// create MariaDB dir
	mariaDBBackupDir := backupDir + "/mariadb/"
	cliutils.CreateDirectory(mariaDBBackupDir, 0755)
	mariadbPass := cliutils.GetSecretValue(ns, kubeClientSet, controllers.MariadbAdminPass)

	if mariadbPass == nil {
		ctrl.Log.Error(errors.New("MariaDB password is missing"),
			"Can not continue. Password in the MariaDB secret is empty!")
		os.Exit(1)
	}

	pod := cliutils.GetPodByName(dbBackupPod, ns, kubeClientSet)

	// NOTE: We use option: --single-transaction to avoid error:
	// "The user specified as a definer ('mariadb.sys'@'localhost') does not exist" when using LOCK TABLES
	backupZuulCMD := []string{
		"mysqldump",
		"-uroot",
		"-p" + *mariadbPass,
		"--databases",
		"zuul",
		"--single-transaction",
	}

	// just create Zuul DB backup
	commandBuffer := cliutils.RunRemoteCmd(kubeContext, ns, pod.Name, controllers.MariaDBIdent, backupZuulCMD)

	// write stdout to file
	cliutils.WriteContentToFile(mariaDBBackupDir+"db-zuul.sql", commandBuffer.Bytes(), 0640)
	ctrl.Log.Info("Finished doing DBs backup!")
}

func backupCmd(kmd *cobra.Command, args []string) {
	backupSecrets, _ := kmd.Flags().GetBool("secrets")
	backupZuul, _ := kmd.Flags().GetBool("zuul")
	backupDB, _ := kmd.Flags().GetBool("db")
	backupAll, _ := kmd.Flags().GetBool("all")
	backupDir, _ := kmd.Flags().GetString("backup_dir")

	if backupDir == "" {
		ctrl.Log.Error(errors.New("no backup dir set"), "You need to set --backup_dir parameter!")
		os.Exit(1)
	}

	if backupSecrets || backupZuul || backupDB {
		backupAll = false
	}

	// prepare to make backup
	ns, kubeClientSet, kubeContext := prepareBackup(kmd, backupDir)

	if ns == "" {
		ctrl.Log.Error(errors.New("no namespace set"), "You need to specify the namespace!")
		os.Exit(1)
	}

	ctrl.Log.Info("Starting backup process for services in namespace: " + ns)

	// create secret backup
	if backupSecrets || backupAll {
		createSecretBackup(ns, backupDir, kubeClientSet)
	}

	// create zuul backup
	if backupZuul || backupAll {
		createZuulKeypairBackup(ns, backupDir, kubeClientSet, kubeContext)
	}

	// create DB backup
	if backupDB || backupAll {
		createMySQLBackup(ns, backupDir, kubeClientSet, kubeContext)
	}

}

func MkBackupCmd() *cobra.Command {

	var (
		backupDir     string
		backupSecrets bool
		backupZuul    bool
		backupDB      bool
		backupAll     bool
		backupCmd     = &cobra.Command{
			Use:   "backup",
			Short: "Create a backup of a deployment",
			Long:  `This command will do a backup of important resources`,
			Run:   backupCmd,
		}
	)

	backupCmd.Flags().StringVar(&backupDir, "backup_dir", "", "The path to the backup directory")
	backupCmd.Flags().BoolVar(&backupSecrets, "secrets", false, "Enable backup for secrets")
	backupCmd.Flags().BoolVar(&backupZuul, "zuul", false, "Enable backup for Zuul private keys")
	backupCmd.Flags().BoolVar(&backupDB, "db", false, "Enable backup for DB's")
	backupCmd.Flags().BoolVar(&backupAll, "all", true, "Make backup of all important content")
	return backupCmd
}
