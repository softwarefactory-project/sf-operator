// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the config job configuration.

package controllers

import (
	_ "embed"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
)

//go:embed static/sf_operator/secret.py
var pymod_secret string

//go:embed static/sf_operator/main.py
var pymod_main string

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
			Image:   POST_INIT_IMAGE,
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
		"secret.py": pymod_secret,
		"main.py":   pymod_main,
	})
}

func (r *SFController) SetupConfigJob() bool {
	r.InstallTooling()
	return r.SetupBaseSecret()
}
