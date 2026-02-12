// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"os"
	"path/filepath"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

const (
	zuulBackupPod     = "zuul-kazoo"
	dbBackupPod       = "mariadb-0"
	DBBackupPath      = "mariadb/db-zuul.sql"
	ZuulBackupPath    = "zuul/zuul.keys"
	SecretsBackupPath = "secrets/"
)

var SecretsToBackup = []string{
	"zookeeper-client-tls",
	"zookeeper-server-tls",
	"nodepool-builder-ssh-key",
	"zuul-ssh-key",
	"zuul-keystore-password",
	"zuul-auth-secret",
	"logserver-keys",
}

func (r *SFKubeContext) createSecretBackup(backupDir string, cr sfv1.SoftwareFactory) error {
	ctrl.Log.Info("Creating secrets backup...")

	secretsDir := backupDir + "/" + SecretsBackupPath
	if err := os.MkdirAll(secretsDir, 0750); err != nil {
		ctrl.Log.Error(err, "Couldn't create backup dir:"+secretsDir)
		return err
	}

	for _, secName := range append(SecretsToBackup, CRSecrets(cr)...) {
		secret := apiv1.Secret{}
		r.GetM(secName, &secret)

		// create new map with important content
		cleanSecret := apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secret.Name, Namespace: secret.Namespace, Annotations: secret.Annotations},
			Data:       secret.Data,
		}
		// dump to yaml
		yamlData, err := yaml.Marshal(cleanSecret)
		if err != nil {
			ctrl.Log.Error(err, "Can not dump to yaml for: "+secName)
			return err
		}

		// write to file
		if err := os.WriteFile(secretsDir+"/"+secret.Name+".yaml", yamlData, 0640); err != nil {
			ctrl.Log.Error(err, "Couldn't write: "+secret.Name)
			return err
		}
	}
	ctrl.Log.Info("Finished doing secret backup!")
	return nil
}

func (r *SFKubeContext) createZuulKeypairBackup(backupDir string) error {

	ctrl.Log.Info("Doing Zuul keys backup...")

	// https://zuul-ci.org/docs/zuul/latest/client.html
	zuulBackupPath := backupDir + "/" + ZuulBackupPath
	zuulBackupDir := filepath.Dir(zuulBackupPath)

	if err := os.MkdirAll(zuulBackupDir, 0750); err != nil {
		ctrl.Log.Error(err, "Couldn't create backup dir:"+zuulBackupDir)
		return err
	}

	backupZuulCMD := []string{
		"zuul-admin",
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

	WaitFor(r.EnsureKazooPod)
	defer r.DeleteKazooPod()

	// Execute command for backup
	r.PodExecM("zuul-kazoo", "zuul-kazoo", backupZuulCMD)

	// Take output of the backup
	commandBuffer := r.PodExecBytes("zuul-kazoo", "zuul-kazoo", backupZuulPrintCMD)

	// write stdout to file
	if err := os.WriteFile(zuulBackupPath, commandBuffer.Bytes(), 0640); err != nil {
		ctrl.Log.Error(err, "Couldn't write: "+zuulBackupPath)
		return err
	}

	// Remove key file from the pod
	r.PodExecM("zuul-kazoo", "zuul-kazoo", backupZuulRemoveCMD)

	ctrl.Log.Info("Finished doing Zuul private keys backup!")
	return nil
}

func (r *SFKubeContext) createMySQLBackup(backupDir string) error {
	ctrl.Log.Info("Doing DB backup...")

	// create MariaDB dir
	mariadbBackupPath := backupDir + "/" + DBBackupPath
	mariaDBBackupDir := filepath.Dir(mariadbBackupPath)

	if err := os.MkdirAll(mariaDBBackupDir, 0750); err != nil {
		ctrl.Log.Error(err, "Couldn't create backup dir:"+mariaDBBackupDir)
		return err
	}

	pod := apiv1.Pod{}
	r.GetM(dbBackupPod, &pod)

	// NOTE: We use option: --single-transaction to avoid error:
	// "The user specified as a definer ('mariadb.sys'@'localhost') does not exist" when using LOCK TABLES
	backupZuulCMD := []string{
		"mysqldump",
		"--databases",
		"zuul",
		"--single-transaction",
	}

	// just create Zuul DB backup
	commandBuffer := r.PodExecBytes(pod.Name, MariaDBIdent, backupZuulCMD)

	// write stdout to file
	if err := os.WriteFile(mariadbBackupPath, commandBuffer.Bytes(), 0640); err != nil {
		ctrl.Log.Error(err, "Couldn't write:"+mariadbBackupPath)
		return err
	}
	ctrl.Log.Info("Finished doing DBs backup!")
	return nil
}

func (r *SFKubeContext) DoBackup(backupDir string, cr sfv1.SoftwareFactory) error {
	// TODO: check that the CR name and the FQDN match the cr being backuped
	ctrl.Log.Info("Starting backup process for services in namespace: " + r.Ns)

	// create secret backup
	if err := r.createSecretBackup(backupDir, cr); err != nil {
		return err
	}

	// create zuul backup
	if err := r.createZuulKeypairBackup(backupDir); err != nil {
		return err
	}

	// create DB backup
	if err := r.createMySQLBackup(backupDir); err != nil {
		return err
	}
	return nil
}

// restore
func (r *SFKubeContext) restoreSecret(backupDir string, cr sfv1.SoftwareFactory) error {
	ctrl.Log.Info("Restoring secrets...")

	for _, sec := range append(SecretsToBackup, CRSecrets(cr)...) {
		pathToSecret := backupDir + "/" + SecretsBackupPath + "/" + sec + ".yaml"
		data, err := os.ReadFile(pathToSecret)
		if err != nil {
			ctrl.Log.Error(err, "Couldn't read secret: "+pathToSecret)
			return err
		}
		var secret apiv1.Secret
		if err := yaml.Unmarshal(data, &secret); err != nil {
			ctrl.Log.Error(err, "Couldn't decode secret: "+pathToSecret)
			return err
		}
		secret.SetNamespace(r.Ns)
		r.CreateR(&secret)
	}
	return nil
}

func (r *SFKubeContext) restoreDB(backupDir string) error {
	ctrl.Log.Info("Restoring DB...")
	pod := apiv1.Pod{}
	r.GetM(dbBackupPod, &pod)

	dropDBCMD := []string{
		"mysql",
		"-e DROP DATABASE zuul;",
	}
	r.PodExecM(pod.Name, MariaDBIdent, dropDBCMD)

	data, err := os.ReadFile(backupDir + "/" + DBBackupPath)
	if err != nil {
		ctrl.Log.Error(err, "Couldn't read sql dump")
		return err
	}

	err = r.PodExecIn("mariadb-0", "mariadb", []string{"mysql", "-h0"}, bytes.NewReader(data))
	if err != nil {
		ctrl.Log.Error(err, "Couldn't inject sql dump")
		return err
	}

	ctrl.Log.Info("Finished restoring DB from backup!")
	return nil
}

func (r *SFKubeContext) restoreZuul(backupDir string) error {
	ctrl.Log.Info("Restoring Zuul...")

	// ensure that pod does not have any restore file
	cleanCMD := []string{
		"bash", "-c", "rm -rf /tmp/zuul-import && mkdir -p /tmp/zuul-import"}
	r.PodExecM("zuul-kazoo", "zuul-kazoo", cleanCMD)

	// copy the Zuul private keys backup to pod
	data, err := os.ReadFile(backupDir + "/" + ZuulBackupPath)
	if err != nil {
		ctrl.Log.Error(err, "Couldn't read zuul.keys")
		return err
	}

	// https://zuul-ci.org/docs/zuul/latest/client.html
	restoreCMD := []string{
		"bash", "-c", "cat > /tmp/zuul.keys && zuul-admin import-keys --force /tmp/zuul.keys"}

	// Execute command for restore
	err = r.PodExecIn("zuul-kazoo", "zuul-kazoo", restoreCMD, bytes.NewReader(data))
	if err != nil {
		ctrl.Log.Error(err, "Couldn't inject zuul.keys")
		return err
	}

	ctrl.Log.Info("Finished doing Zuul private keys restore!")
	return nil
}

func (r *SFKubeContext) DoRestore(backupDir string, cr sfv1.SoftwareFactory) error {
	if err := r.restoreSecret(backupDir, cr); err != nil {
		return err
	}

	ctrl.Log.Info("Spawning backend services...")
	sfCtrl := MkSFController(*r, cr)
	sfCtrl.DeployMariadb()
	sfCtrl.DeployZookeeper()
	ctrl.Log.Info("Waiting for backend services...")
	WaitFor(sfCtrl.DeployMariadb)
	WaitFor(sfCtrl.DeployZookeeper)

	sfCtrl.DeployZuulSecrets()
	sfCtrl.EnsureZuulConfigSecret(false)
	sfCtrl.EnsureToolingVolume()
	WaitFor(sfCtrl.EnsureKazooPod)

	if err := r.restoreZuul(backupDir); err != nil {
		return err
	}
	if err := r.restoreDB(backupDir); err != nil {
		return err
	}

	sfCtrl.DeleteKazooPod()

	// Run deployment to ensure everything is running as expected.
	if err := r.StandaloneReconcile(cr); err != nil {
		ctrl.Log.Error(err, "Reconcille failed")
		return err
	}
	return nil
}
