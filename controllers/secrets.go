// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"

	_ "embed"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	apiv1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

//go:embed static/rotate-projects-private-keys.py
var rotateProjectsPrivateKeys string

func (r *SFKubeContext) RotateProjectPrivateKey(sshKey string, unixAge int64, authorName string, authorMail string) error {
	var err error
	WaitFor(r.EnsureKazooPod, false)

	// Clear config state to ensure the internal git is refreshed
	r.ClearConfigJob()
	r.DeleteR(&apiv1.ConfigMap{ObjectMeta: r.MkMeta("zs-internal-tenant-reconfigure")})
	r.EnsureConfigMap("zk-clients-need-refresh", map[string]string{})
	r.EnsureConfigMap("zuul-needs-full-reconfigure", map[string]string{})

	// Copy the rotation script
	err = r.PodExecIn("zuul-kazoo", "zuul-kazoo", []string{"bash", "-c", "cat > /tmp/rotate-projects-private-keys.py && chmod 755 /tmp/*.py"}, bytes.NewReader([]byte(rotateProjectsPrivateKeys)))
	if err != nil {
		ctrl.Log.Error(err, "Couldn't install rotation script")
		return err
	}

	// Copy the ssh key
	data, err := os.ReadFile(sshKey)
	if err != nil {
		ctrl.Log.Error(err, "Couldn't read ssh key")
		return err
	}
	err = r.PodExecIn("zuul-kazoo", "zuul-kazoo", []string{"bash", "-c", "cat > /var/lib/zuul/.ssh_push_key && chmod 0600 /var/lib/zuul/.ssh_push_key"}, bytes.NewReader(data))
	if err != nil {
		ctrl.Log.Error(err, "Couldn't install ssh key")
		return err
	}

	// Copy the tenants config
	tenants, err := r.PodExecBytes("zuul-scheduler-0", "zuul-scheduler", []string{"cat", "/var/lib/zuul/main.yaml"})
	if err != nil {
		ctrl.Log.Error(err, "Couldn't read tenants config")
		return err
	}
	err = r.PodExecIn("zuul-kazoo", "zuul-kazoo", []string{"bash", "-c", "cat > /var/lib/zuul/main.yaml"}, bytes.NewReader(tenants.Bytes()))
	if err != nil {
		ctrl.Log.Error(err, "Couldn't install tenants config")
		return err
	}

	// Grab the logserver key
	logserverKey := base64.StdEncoding.EncodeToString([]byte(r.ReadSecretValue("zuul-ssh-key", "priv")))

	if unixAge == 0 {
		// max age
		unixAge = 9223372036854775807
	}
	return r.PodExec("zuul-kazoo", "zuul-kazoo", []string{"env", "PYTHONUNBUFFERED=1", "/tmp/rotate-projects-private-keys.py", "--age", strconv.FormatInt(unixAge, 10), "--author", authorName, "--email", authorMail, "--logserver-key", logserverKey})
}

func (r *SFKubeContext) _DeleteSecretOrError(name string) error {
	// Delete and recreate
	var secret apiv1.Secret
	if !r.GetOrDie(name, &secret) {
		return errors.New("missing secret: " + name)
	}
	r.DeleteR(&secret)
	return nil
}

// RotateZookeeperTLSSecrets requires a reconcile, stopping executors prior to the rotation is advised
func (r *SFKubeContext) rotateZookeeperTLSSecrets() error {
	var secretClient apiv1.Secret
	var secretServer apiv1.Secret
	if !r.GetOrDie("zookeeper-server-tls", &secretServer) {
		return errors.New("missing zookeeper server secret")
	}
	if !r.GetOrDie("zookeeper-client-tls", &secretClient) {
		return errors.New("missing zookeeper client secret")
	}
	r.DeleteR(&secretClient)
	r.DeleteR(&secretServer)
	return nil
}

// RotateZuulDBConnectionSecret requires a reconcile to regenerate the credentials and update the database
func (r *SFKubeContext) rotateZuulDBConnectionSecret() error {
	return r._DeleteSecretOrError("zuul-db-connection")
}

// RotateZuulAuthenticatorSecret requires a restart of zuul-web and zuul-scheduler
func (r *SFKubeContext) rotateZuulAuthenticatorSecret() error {
	// Delete and recreate
	if err := r._DeleteSecretOrError("zuul-auth-secret"); err != nil {
		return err
	}
	r.EnsureSecretUUID("zuul-auth-secret")
	return nil

}

// RotateKeystorePassword requires a restart of zuul-web and zuul-scheduler
func (r *SFKubeContext) rotateKeystorePassword() error {
	// Check resources
	var secret apiv1.Secret
	if r.GetOrDie("zuul-keystore-password-new", &secret) {
		return errors.New("existing zuul-keystore-password-new found")
	}
	if !r.GetOrDie("zuul-keystore-password", &secret) {
		return errors.New("missing zuul-keystore-password secret")
	}
	tmpSecret := r.EnsureSecretUUID("zuul-keystore-password-new")

	// Setup new password
	oldPassword := string(secret.Data["zuul-keystore-password"])
	newPassword := string(tmpSecret.Data["zuul-keystore-password-new"])

	WaitFor(r.EnsureKazooPod, false)
	defer r.DeleteKazooPod()

	// Perform rotation
	if _, err := r.RunPodCmd("zuul-kazoo", "zuul-kazoo", []string{"python3", "/usr/local/bin/rotate-keystore.py", oldPassword, newPassword}); err != nil {
		return err
	}
	keys, err := r.PodExecBytes("zuul-kazoo", "zuul-kazoo", []string{"cat", "/var/lib/zuul/keys-backup.json"})
	if err != nil {
		ctrl.Log.Error(err, "Couldn't read previous keys")
	} else {
		os.WriteFile("zuul-keys.backup", keys.Bytes(), 0600)
		os.WriteFile("zuul-keys.password", []byte(oldPassword), 0600)
	}

	// Update secret
	secret.Data["zuul-keystore-password-old"] = secret.Data["zuul-keystore-password"]
	secret.Data["zuul-keystore-password"] = []byte(newPassword)
	if !r.UpdateR(&secret) {
		return errors.New("couldn't save the new secret")
	}
	r.DeleteR(&tmpSecret)
	return nil
}

func (r *SFKubeContext) DoRotateSecrets() error {
	var podList apiv1.PodList
	if err := r.List(&podList); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, "zuul-executor") {
			logging.LogW("At least one executor running, this may cause issues when rotating Zookeeper secrets.")
			break
		}
	}
	logging.LogI("Rotating Keystore password...")
	if err := r.rotateKeystorePassword(); err != nil {
		return err
	}
	logging.LogI("Killing every kazoo client...")
	r.nukeZK()

	logging.LogI("Rotating Zuul Client Authenticator secret...")
	if err := r.rotateZuulAuthenticatorSecret(); err != nil {
		return err
	}
	logging.LogI("Rotating Zuul DB Connection secret...")
	if err := r.rotateZuulDBConnectionSecret(); err != nil {
		return err
	}
	logging.LogI("Rotating Zookeeper certificates...")
	if err := r.rotateZookeeperTLSSecrets(); err != nil {
		return err
	}
	logging.LogI("Force Restart impacted services...")
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, "zuul-scheduler") || strings.HasPrefix(pod.Name, "zuul-web-") {
			r.DeleteR(&pod)
		}
	}
	return nil
}
