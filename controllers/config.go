// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the config job configuration.

package controllers

import (
	_ "embed"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
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

func (r *SFController) SetupBaseSecret() bool {
	var job batchv1.Job
	job_name := "config-base-secret"
	found := r.GetM(job_name, &job)

	if !found {
		r.log.V(1).Info("Creating base secret job")
		r.CreateR(r.RunCommand(job_name, []string{"config-create-k8s-secret"}))
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for base secret job result")
		return false
	}
}

func (r *SFController) RunCommand(name string, args []string) *batchv1.Job {
	job := create_job(
		r.ns, name,
		apiv1.Container{
			Name:    "sf-operator",
			Image:   BUSYBOX_IMAGE,
			Command: append([]string{"python3", "/sf_operator/main.py"}, args...),
			Env: []apiv1.EnvVar{
				create_env("PYTHONPATH", "/"),
			},
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

func (r *SFController) SetupConfigRepo(config_repo_url string, config_repo_user string, gerrit_enabled bool) bool {
	r.InstallTooling()
	var job batchv1.Job
	job_name := "setup-config-repo"
	found := r.GetM(job_name, &job)

	if !found {
		job := create_job(
			r.ns, job_name,
			apiv1.Container{
				Name:    "sf-operator",
				Image:   BUSYBOX_IMAGE,
				Command: append([]string{"bash", "/sf_operator/config-repo.sh"}),
				Env: []apiv1.EnvVar{
					create_secret_env("SF_ADMIN_SSH", "admin-ssh-key", "priv"),
					create_env("FQDN", r.cr.Spec.FQDN),
					create_env("CONFIG_REPO_URL", config_repo_url),
					create_env("CONFIG_REPO_USER", config_repo_user),
					create_env("GERRIT_ENABLED", strconv.FormatBool(gerrit_enabled)),
				},
				VolumeMounts: []apiv1.VolumeMount{
					{Name: "pymod-sf-operator", MountPath: "/sf_operator"},
				},
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
