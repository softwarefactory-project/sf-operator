// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"errors"
	"strings"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	apiv1 "k8s.io/api/core/v1"
)

// RotateZuulDBConnectionSecret requires a reconcile to regenerate the credentials and update the database
func (r *SFKubeContext) rotateZuulDBConnectionSecret() error {
	// Delete and recreate
	var secret apiv1.Secret
	if !r.GetM("zuul-db-connection", &secret) {
		return errors.New("missing zuul-db-connection secret")
	}
	r.DeleteR(&secret)
	return nil
}

// RotateZuulAuthenticatorSecret requires a restart of zuul-web and zuul-scheduler
func (r *SFKubeContext) rotateZuulAuthenticatorSecret() error {
	// Delete and recreate
	var secret apiv1.Secret
	if !r.GetM("zuul-auth-secret", &secret) {
		return errors.New("missing zuul-auth-secret secret")
	}
	r.DeleteR(&secret)
	r.EnsureSecretUUID("zuul-auth-secret")

	return nil

}

// RotateKeystorePassword requires a restart of zuul-web and zuul-scheduler
func (r *SFKubeContext) rotateKeystorePassword() error {
	// Check resources
	var secret apiv1.Secret
	if r.GetM("zuul-keystore-password-new", &secret) {
		return errors.New("existing zuul-keystore-password-new found")
	}
	if !r.GetM("zuul-keystore-password", &secret) {
		return errors.New("missing zuul-keystore-password secret")
	}
	tmpSecret := r.EnsureSecretUUID("zuul-keystore-password-new")

	// Setup new password
	oldPassword := string(secret.Data["zuul-keystore-password"])
	newPassword := string(tmpSecret.Data["zuul-keystore-password-new"])

	// Perform rotation
	if _, err := r.RunPodCmd("zuul-scheduler-0", "zuul-scheduler", []string{"python3", "/usr/local/bin/rotate-keystore.py", oldPassword, newPassword}); err != nil {
		return err
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

	logging.LogI("Rotating Keystore password...")
	if err := r.rotateKeystorePassword(); err != nil {
		return err
	}
	logging.LogI("Rotating Zuul Client Authenticator secret...")
	if err := r.rotateZuulAuthenticatorSecret(); err != nil {
		return err
	}
	logging.LogI("Rotating Zuul DB Connection secret...")
	if err := r.rotateZuulDBConnectionSecret(); err != nil {
		return err
	}
	logging.LogI("Force Restart impacted services...")
	var podList apiv1.PodList
	if err := r.Client.List(r.Ctx, &podList); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, "zuul-scheduler") || strings.HasPrefix(pod.Name, "zuul-web-") {
			r.DeleteR(&pod)
		}
	}
	return nil
}
