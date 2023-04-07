// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the managesf-resources deployment configuration.
package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
)

const MANAGESF_RESOURCES_IDENT string = "managesf-resources"

//go:embed static/managesf-resources/entrypoint.sh
var managesf_entrypoint string

func GenerateConfig(r *SFController) (string, error) {

	// Getting Gerrit Admin password from secret

	gerritadminpassword, err := r.getSecretData("gerrit-admin-api-key")
	if err != nil {
		return "", err
	}

	// Structure for config.py file template
	type ConfigPy struct {
		Fqdn                string
		GerritAdminPassword string
	}

	// Initializing Template Structure
	configpy := ConfigPy{
		r.cr.Spec.FQDN,
		string(gerritadminpassword),
	}

	// Template path
	templatefile := "controllers/static/managesf-resources/config.py.tmpl"

	template, err := parse_template(templatefile, configpy)
	if err != nil {
		r.log.V(1).Error(err, "Template parsing failed")
		return "", err
	}

	return template, nil
}

func (r *SFController) DeployManagesfResources() bool {

	// Creating managesf config.py file
	config_data := make(map[string]string)
	var err error
	config_data["config.py"], err = GenerateConfig(r)
	if err != nil {
		r.log.V(1).Error(err, "Unable to generate managesf config file")
		return false
	}
	r.EnsureConfigMap(MANAGESF_RESOURCES_IDENT, config_data)

	// Create the deployment object
	dep := create_deployment(r.ns, MANAGESF_RESOURCES_IDENT, BUSYBOX_IMAGE)
	dep.Spec.Template.Spec.SecurityContext = &defaultPodSecurityContext

	// Amend the deployment's container
	dep.Spec.Template.Spec.Containers[0].Command = []string{"bash", "-c", managesf_entrypoint}
	dep.Spec.Template.Spec.Containers[0].SecurityContext = &defaultContainerSecurityContext
	dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		// managesf-resources need an admin ssh access to the local Gerrit
		create_secret_env("SF_ADMIN_SSH", "admin-ssh-key", "priv"),
	}
	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_cmd_probe([]string{"cat", "/tmp/healthy"})
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      MANAGESF_RESOURCES_IDENT + "-config-vol",
			MountPath: "/etc/managesf",
		},
		// managesf-resources command uses (by default) this directory for its cache
		{
			Name:      MANAGESF_RESOURCES_IDENT + "-cache",
			MountPath: "/var/lib/software-factory",
		},
	}

	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		create_empty_dir(MANAGESF_RESOURCES_IDENT + "-cache"),
		create_volume_cm(MANAGESF_RESOURCES_IDENT+"-config-vol", MANAGESF_RESOURCES_IDENT+"-config-map"),
	}

	r.GetOrCreate(&dep)

	return r.IsDeploymentReady(&dep)
}
