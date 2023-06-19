// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the config job configuration.

package controllers

import (
	_ "embed"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed static/sf_operator/secret.py
var pymod_secret string

//go:embed static/sf_operator/main.py
var pymod_main string

//go:embed static/sf_operator/config-repo.sh
var config_repo string

//go:embed static/sf_operator/config-updater-sa.yaml
var config_updater_sa string

func (r *SFController) SetupBaseSecrets() bool {

	r.CreateYAMLs(config_updater_sa)

	// Create a long lived service account token for the use within the
	// config-update process
	// See: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account
	var secret apiv1.Secret
	secret_name := "config-update-secrets"
	if !r.GetM(secret_name, &secret) {
		r.log.V(1).Info("Creating the config-update service account secret")
		secret = apiv1.Secret{
			Type: "kubernetes.io/service-account-token",
			ObjectMeta: metav1.ObjectMeta{
				Name: secret_name,
				Annotations: map[string]string{
					"kubernetes.io/service-account.name": "config-updater"},
				Namespace: r.ns,
			},
		}
		r.CreateR(&secret)
	}

	var job batchv1.Job
	job_name := "config-base-secret"
	found := r.GetM(job_name, &job)

	extra_cmd_vars := []apiv1.EnvVar{
		create_env("HOME", "/tmp"),
		create_secret_env("SERVICE_ACCOUNT_TOKEN", secret_name, "token"),
		create_secret_env("ZUUL_LOGSERVER_PRIVATE_KEY", "zuul-ssh-key", "priv"),
	}

	if !found {
		r.log.V(1).Info("Creating base secret job")
		r.CreateR(r.RunCommand(job_name, []string{"config-create-zuul-secrets"}, extra_cmd_vars))
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for base secret job result")
		return false
	}
}

func (r *SFController) RunCommand(name string, args []string, extra_vars []apiv1.EnvVar) *batchv1.Job {
	job := r.create_job(
		name,
		apiv1.Container{
			Name:    "sf-operator",
			Image:   BUSYBOX_IMAGE,
			Command: append([]string{"python3", "/sf_operator/main.py"}, args...),
			Env: append([]apiv1.EnvVar{
				create_env("PYTHONPATH", "/"),
				create_env("FQDN", r.cr.Spec.FQDN),
			}, extra_vars...),
			VolumeMounts: []apiv1.VolumeMount{
				{Name: "pymod-sf-operator", MountPath: "/sf_operator"},
			},
			SecurityContext: create_security_context(false),
		},
	)
	job.Spec.Template.Spec.Volumes = []apiv1.Volume{
		create_volume_cm("pymod-sf-operator", "pymod-sf-operator-config-map"),
	}
	return &job
}

func (r *SFController) InstallTooling() {
	r.EnsureConfigMap("pymod-sf-operator", map[string]string{
		"secret.py":      pymod_secret,
		"main.py":        pymod_main,
		"config-repo.sh": config_repo,
	})
}

func (r *SFController) SetupConfigJob() bool {
	r.InstallTooling()
	if r.SetupBaseSecrets() {
		// We run zuul full-reconfigure once to ensure that the zuul scheduler loaded the provisionned config
		var zs_bootstrap_full_reconfigure apiv1.ConfigMap
		var cm_name = "zs-bootstrap-full-reconfigure"
		if !r.GetM(cm_name, &zs_bootstrap_full_reconfigure) {
			r.log.Info("Running the initial zuul-scheduler full-reconfigure")
			if r.runZuulFullReconfigure() {
				// Create an empty ConfigMap to keep note the reconfigure has been already done
				zs_bootstrap_full_reconfigure.ObjectMeta = metav1.ObjectMeta{
					Name:      cm_name,
					Namespace: r.ns,
				}
				zs_bootstrap_full_reconfigure.Data = map[string]string{}
				r.CreateR(&zs_bootstrap_full_reconfigure)
			}
		} else {
			return true
		}
	}
	return false
}

func (r *SFController) SetupConfigRepo() bool {
	r.InstallTooling()
	var job batchv1.Job
	job_name := "setup-config-repo"
	found := r.GetM(job_name, &job)

	if !found {
		config_url, config_user := r.getConfigRepoCNXInfo()
		job := r.create_job(
			job_name,
			apiv1.Container{
				Name:    "sf-operator",
				Image:   BUSYBOX_IMAGE,
				Command: append([]string{"bash", "/sf_operator/config-repo.sh"}),
				Env: []apiv1.EnvVar{
					create_secret_env("SF_ADMIN_SSH", "admin-ssh-key", "priv"),
					create_env("FQDN", r.cr.Spec.FQDN),
					create_env("CONFIG_REPO_URL", config_url),
					create_env("CONFIG_REPO_USER", config_user),
					create_env("HOME", "/tmp"),
				},
				VolumeMounts: []apiv1.VolumeMount{
					{Name: "pymod-sf-operator", MountPath: "/sf_operator"},
				},
				SecurityContext: create_security_context(false),
			},
		)
		job.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm("pymod-sf-operator", "pymod-sf-operator-config-map"),
		}
		r.log.V(1).Info("Populating config-repo")
		r.CreateR(&job)
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for the setup-config-repo job result")
		return false
	}
}
