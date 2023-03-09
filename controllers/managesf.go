// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the managesf configuration.
package controllers

import (
	"bytes"
	_ "embed"
	"text/template"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const MANAGESF_IDENT string = "managesf"
const MANAGESF_IMAGE string = "quay.io/software-factory/managesf:0.30.0-1"

const MANAGESF_PORT = 20001
const MANAGESF_PORT_NAME = "managesfport"

func GenerateConfig(sqlsecret apiv1.Secret, r *SFController) string {

	// Getting Gerrit Admin password from secret
	gerritadminpassword := []byte{}
	gerritsecret, err := r.getSecretbyNameRef("gerrit-admin-api-key")
	if err != nil {
		r.log.V(1).Error(err, "gerrit-admin-api-key secret not found")
	}
	gerritadminpassword, err = r.getValueFromKeySecret(gerritsecret, "gerrit-admin-api-key")
	if err != nil {
		r.log.V(1).Error(err, "Key not found")
	}

	// Structure for config.py file template
	type ConfigPy struct {
		ManageSFIdent       string
		Fqdn                string
		GerritAdminPassword string
		ManagesfDBPassword  string
	}

	// Initializing Template Structure
	configpy := ConfigPy{
		MANAGESF_IDENT,
		r.cr.Spec.FQDN,
		string(gerritadminpassword),
		string(sqlsecret.Data["managesf-db-password"]),
	}

	// Template path
	templatefile := "controllers/static/managesf/config.py.tmpl"

	template, err := parse_template(templatefile, configpy)
	if err != nil {
		r.log.V(1).Error(err, "Template parsing failed")
	}

	return template
}

func GenerateSshConfig(sqlsecret apiv1.Secret, r *SFController) string {

	// Structure for SSH config file template
	type SshConfig struct {
		Fqdn string
	}

	// Initializing Template Structure
	sshconfig := SshConfig{r.cr.Spec.FQDN}

	// Template path
	templatefile := "controllers/static/managesf/sshconfig.tmpl"

	// Opening Template file
	template, err := template.ParseFiles(templatefile)
	if err != nil {
		r.log.V(1).Error(err, "File not found")
	}

	// Parsing Template
	var buf bytes.Buffer
	err = template.Execute(&buf, sshconfig)
	if err != nil {
		r.log.V(1).Error(err, "Failure while parsing tamplate %s", templatefile)
	}

	return buf.String()
}

func (r *SFController) DeployManagesf() bool {

	initContainers, managesfpasswordsecret := r.EnsureDBInit("managesf")

	// Creating managesf config.py file
	config_data := make(map[string]string)
	config_data["config.py"] = GenerateConfig(managesfpasswordsecret, r)
	r.EnsureConfigMap(MANAGESF_IDENT, config_data)

	ssh_config_data := make(map[string]string)
	ssh_config_data["config"] = GenerateSshConfig(managesfpasswordsecret, r)
	r.EnsureConfigMap(MANAGESF_IDENT+"-ssh", ssh_config_data)

	dep := create_deployment(r.ns, MANAGESF_IDENT, MANAGESF_IMAGE)

	dep.Spec.Template.Spec.InitContainers = initContainers

	dep.Spec.Template.Spec.Containers[0].Command = []string{
		"managesf.sh"}

	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(MANAGESF_PORT, MANAGESF_PORT_NAME),
	}

	dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      MANAGESF_IDENT + "-config-vol",
			MountPath: "/etc/managesf",
		},
		{
			Name:      MANAGESF_IDENT + "-ssh-config-vol",
			MountPath: "/var/lib/managesf/.ssh",
		},
	}

	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		//create_empty_dir(MANAGESF_IDENT + "-config-vol"),
		create_volume_cm(MANAGESF_IDENT+"-config-vol", MANAGESF_IDENT+"-config-map"),
		create_volume_cm(MANAGESF_IDENT+"-ssh-config-vol", MANAGESF_IDENT+"-ssh-config-map"),
	}

	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(MANAGESF_PORT)

	r.GetOrCreate(&dep)

	srv := create_service(r.ns, MANAGESF_IDENT, MANAGESF_IDENT, MANAGESF_PORT, MANAGESF_PORT_NAME)
	r.GetOrCreate(&srv)

	return r.IsDeploymentReady(&dep)
}

func (r *SFController) IngressManagesf() []netv1.IngressRule {
	return []netv1.IngressRule{
		create_ingress_rule(MANAGESF_IDENT+"."+r.cr.Spec.FQDN, MANAGESF_IDENT, MANAGESF_PORT),
	}
}
