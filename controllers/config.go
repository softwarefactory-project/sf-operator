// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the config job configuration.

package controllers

import (
	_ "embed"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

//go:embed static/sf_operator/secret.py
var pymod_secret string

//go:embed static/sf_operator/main.py
var pymod_main string

//go:embed static/sf_operator/config-repo.sh
var config_repo string

//go:embed static/sf_operator/resources.dhall
var resourcesDhall string

//go:embed static/sf_operator/sf.dhall
var sfDhall string

//go:embed static/sf_operator/config-updater-sa.yaml
var config_updater_sa string

func (r *SFController) SetupBaseSecret() bool {

	r.CreateYAMLs(config_updater_sa)

	// Create a long lived service account token for the use within the
	// config-update process
	// See: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account
	var secret apiv1.Secret
	secret_name := "config-update-secret"
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
		r.CreateR(&secret);
	}

	var job batchv1.Job
	job_name := "config-base-secret"
	found := r.GetM(job_name, &job)

	extra_cmd_vars := []apiv1.EnvVar{
		create_secret_env("SERVICE_ACCOUNT_TOKEN", secret_name, "token")}

	if !found {
		r.log.V(1).Info("Creating base secret job")
		r.CreateR(r.RunCommand(job_name, []string{"config-create-k8s-secret"}, extra_cmd_vars))
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for base secret job result")
		return false
	}
}

func (r *SFController) RunCommand(name string, args []string, extra_vars []apiv1.EnvVar) *batchv1.Job {
	job := create_job(
		r.ns, name,
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
		},
	)
	job.Spec.Template.Spec.Volumes = []apiv1.Volume{
		create_volume_cm("pymod-sf-operator", "pymod-sf-operator-config-map"),
	}
	return &job
}

func (r *SFController) InstallTooling() {
	r.EnsureConfigMap("pymod-sf-operator", map[string]string{
		"secret.py":       pymod_secret,
		"main.py":         pymod_main,
		"config-repo.sh":  config_repo,
		"resources.dhall": resourcesDhall,
		"sf.dhall":        sfDhall,
	})
}

func (r *SFController) SetupConfigJob() bool {
	r.InstallTooling()
	return r.SetupBaseSecret()
}

func (r *SFController) getProvidedCR() string {
	data, err := yaml.Marshal(sfv1.SoftwareFactory{
		TypeMeta: r.cr.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: r.cr.ObjectMeta.CreationTimestamp,
			Name:              r.cr.ObjectMeta.Name,
			Namespace:         r.ns,
		},
		Spec: r.cr.Spec,
	})
	if err != nil {
		panic(err)
	}
	return "# The software factory system resources\n# The config-update job applies change to this file.\n" + string(data)
}

func (r *SFController) SetupConfigRepo() bool {
	r.InstallTooling()
	var job batchv1.Job
	job_name := "setup-config-repo"
	found := r.GetM(job_name, &job)

	// Write the provided cr to a config map for config repo system resource
	r.EnsureConfigMap("sf-provided-cr", map[string]string{
		"sf.yaml": r.getProvidedCR(),
	})

	if !found {
		config_url, config_user := r.getConfigRepoCNXInfo()
		job := create_job(
			r.ns, job_name,
			apiv1.Container{
				Name:    "sf-operator",
				Image:   BUSYBOX_IMAGE,
				Command: append([]string{"bash", "/sf_operator/config-repo.sh"}),
				Env: []apiv1.EnvVar{
					create_secret_env("SF_ADMIN_SSH", "admin-ssh-key", "priv"),
					create_env("FQDN", r.cr.Spec.FQDN),
					create_env("CONFIG_REPO_URL", config_url),
					create_env("CONFIG_REPO_USER", config_user),
				},
				VolumeMounts: []apiv1.VolumeMount{
					{Name: "pymod-sf-operator", MountPath: "/sf_operator"},
					{Name: "sf-provided-cr", MountPath: "/sf-provided-cr"},
				},
			},
		)
		job.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm("pymod-sf-operator", "pymod-sf-operator-config-map"),
			create_volume_cm("sf-provided-cr", "sf-provided-cr-config-map"),
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
