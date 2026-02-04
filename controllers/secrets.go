// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"errors"
	"strings"

	apiv1 "k8s.io/api/core/v1"
)

func (r *SFKubeContext) RotateKeystorePassword() error {
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

	// Restart services
	var podList apiv1.PodList
	if err := r.Client.List(r.Ctx, &podList); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if pod.Name == "zuul-scheduler-0" || strings.HasPrefix(pod.Name, "zuul-web-") {
			r.DeleteR(&pod)
		}
	}
	return nil
}

func (r *SFKubeContext) DoRotateSecrets() error {
	if err := r.RotateKeystorePassword(); err != nil {
		return err
	}

	return nil
}
