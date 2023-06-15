// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

package controllers

import (
	_ "embed"
	"fmt"

	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const GERRIT_IDENT = "gerrit"
const GERRIT_HTTPD_PORT = 8080
const GERRIT_HTTPD_HTTP_PORT = 80
const GERRIT_HTTPD_PORT_NAME = "gerrit-httpd"

const GERRIT_SSHD_PORT = 29418
const GERRIT_SSHD_PORT_NAME = "gerrit-sshd"
const GERRIT_IMAGE = "quay.io/software-factory/gerrit:3.6.4-4"
const GERRIT_EP_MOUNT_PATH = "/entry"
const GERRIT_SITE_MOUNT_PATH = "/gerrit"

const MANAGESF_RESOURCES_IDENT string = "managesf-resources"

//go:embed static/gerrit/post-init.sh
var postInitScript string

//go:embed static/gerrit/entrypoint.sh
var entrypoint string

//go:embed static/gerrit/init.sh
var gerritInitScript string

//go:embed static/managesf-resources/entrypoint.sh
var managesf_entrypoint string

//go:embed static/managesf-resources/config.py.tmpl
var managesf_conf string

func (r *SFController) GenerateManageSFConfig() (string, error) {

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

	template, err := parse_string(managesf_conf, configpy)
	if err != nil {
		panic("Template parsing failed")
	}

	return template, nil
}

func GerritInitContainers(volumeMounts []apiv1.VolumeMount, fqdn string) apiv1.Container {
	container := MkContainer("gerrit-init", GERRIT_IMAGE)
	container.Command = []string{"sh", "-c", gerritInitScript}
	container.Env = []apiv1.EnvVar{
		create_secret_env("GERRIT_ADMIN_SSH_PUB", "admin-ssh-key", "pub"),
		create_env("FQDN", fqdn),
	}
	container.VolumeMounts = volumeMounts
	return container
}

func GerritPostInitContainer(job_name string, fqdn string) apiv1.Container {
	env := []apiv1.EnvVar{
		create_env("HOME", "/tmp"),
		create_env("FQDN", fqdn),
		create_secret_env("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
		create_secret_env("GERRIT_ADMIN_API_KEY", "gerrit-admin-api-key", "gerrit-admin-api-key"),
		create_secret_env("ZUUL_SSH_PUB_KEY", "zuul-ssh-key", "pub"),
		create_secret_env("ZUUL_HTTP_PASSWORD", "zuul-gerrit-api-key", "zuul-gerrit-api-key"),
	}

	container := MkContainer(fmt.Sprintf("%s-container", job_name), BUSYBOX_IMAGE)
	container.Command = []string{"sh", "-c", postInitScript}
	container.Env = env
	return container
}

func (r *SFController) GerritPostInitJob(name string) bool {
	var job batchv1.Job
	job_name := GERRIT_IDENT + "-" + name
	found := r.GetM(job_name, &job)

	if !found {
		container := GerritPostInitContainer(job_name, r.cr.Spec.FQDN)
		job := MkJob(job_name, r.ns, container)
		r.log.V(1).Info("Creating Gerrit post init job", "name", name)
		r.CreateR(&job)
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for Gerrit post init job result")
		return false
	}
}

func GerritHttpdService(ns string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GERRIT_HTTPD_PORT_NAME,
			Namespace: ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:       GERRIT_HTTPD_PORT_NAME,
					Protocol:   apiv1.ProtocolTCP,
					Port:       GERRIT_HTTPD_PORT,
					TargetPort: intstr.FromString(GERRIT_HTTPD_PORT_NAME),
				},
				{
					Name:       GERRIT_HTTPD_PORT_NAME + "-internal-http",
					Protocol:   apiv1.ProtocolTCP,
					Port:       GERRIT_HTTPD_HTTP_PORT,
					TargetPort: intstr.FromString(GERRIT_HTTPD_PORT_NAME),
				},
			},
			Selector: map[string]string{
				"app": "sf",
				"run": GERRIT_IDENT,
			},
		}}
}

func GerritSshdService(ns string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GERRIT_SSHD_PORT_NAME,
			Namespace: ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:     GERRIT_SSHD_PORT_NAME,
					Protocol: apiv1.ProtocolTCP,
					Port:     GERRIT_SSHD_PORT,
				},
			},
			Type: apiv1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "sf",
				"run": GERRIT_IDENT,
			},
		}}
}

func SetGerritSTSContainer(sts *appsv1.StatefulSet, volumeMounts []apiv1.VolumeMount, fqdn string) {
	sts.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", entrypoint}
	sts.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	sts.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME),
		create_container_port(GERRIT_SSHD_PORT, GERRIT_SSHD_PORT_NAME),
	}
	sts.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		create_env("HOME", "/gerrit"),
		create_env("FQDN", fqdn),
		create_secret_env("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
	}
	sts.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_cmd_probe([]string{"bash", "/gerrit/ready.sh"})
}

func SetGerritMSFRContainer(sts *appsv1.StatefulSet) {
	container := MkContainer(MANAGESF_RESOURCES_IDENT, BUSYBOX_IMAGE)
	container.Command = []string{"sh", "-c", managesf_entrypoint}
	container.Env = []apiv1.EnvVar{
		create_env("HOME", "/var/lib/managesf"),
		// managesf-resources need an admin ssh access to the local Gerrit
		create_secret_env("SF_ADMIN_SSH", "admin-ssh-key", "priv"),
	}
	container.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      MANAGESF_RESOURCES_IDENT + "-config-vol",
			MountPath: "/etc/managesf",
		},
		// managesf-resources command uses (by default) this directory for its cache
		{
			Name:      MANAGESF_RESOURCES_IDENT + "-cache",
			MountPath: "/var/lib/software-factory",
		},
		{
			Name:      MANAGESF_RESOURCES_IDENT + "-home",
			MountPath: "/var/lib/managesf",
		},
	}
	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, container)
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes,
		create_empty_dir(MANAGESF_RESOURCES_IDENT+"-home"),
		create_empty_dir(MANAGESF_RESOURCES_IDENT+"-cache"),
		create_volume_cm(MANAGESF_RESOURCES_IDENT+"-config-vol", MANAGESF_RESOURCES_IDENT+"-config-map"),
	)
}

func (r *SFController) DeployGerrit() bool {

	// Ensure Gerrit Admin API password
	r.GenerateSecretUUID("gerrit-admin-api-key")

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      GERRIT_IDENT,
			MountPath: GERRIT_SITE_MOUNT_PATH,
		},
	}

	// Creating managesf config.py file
	config_data := make(map[string]string)
	var err error
	config_data["config.py"], err = r.GenerateManageSFConfig()
	if err != nil {
		r.log.V(1).Error(err, "Unable to generate managesf config file")
		return false
	}
	r.EnsureConfigMap(MANAGESF_RESOURCES_IDENT, config_data)

	// Create the deployment
	dep := r.create_statefulset(
		GERRIT_IDENT,
		GERRIT_IMAGE,
		BaseGetStorageConfOrDefault(v1.StorageSpec{}, ""),
		int32(1))

	// Setup the main container
	SetGerritSTSContainer(&dep, volumeMounts, r.cr.Spec.FQDN)

	// Setup the init container
	dep.Spec.Template.Spec.InitContainers = []apiv1.Container{GerritInitContainers(volumeMounts, r.cr.Spec.FQDN)}

	// Setup the managesf-resources sidecar container
	SetGerritMSFRContainer(&dep)

	// Create annotations based on Gerrit parameters
	annotations := map[string]string{
		"fqdn": r.cr.Spec.FQDN,
		// The serial is a just a way to trigger rollout
		"serial": "1",
	}
	dep.Spec.Template.ObjectMeta.Annotations = annotations
	r.GetOrCreate(&dep)
	if !map_equals(&dep.Spec.Template.ObjectMeta.Annotations, &annotations) {
		// Update the annotation - this force the statefulset controler to respawn the container
		dep.Spec.Template.ObjectMeta.Annotations = annotations
		// ReInit initContainers to ensure new spec is used
		dep.Spec.Template.Spec.InitContainers = []apiv1.Container{GerritInitContainers(volumeMounts, r.cr.Spec.FQDN)}
		// Ensure the managesf-ressource container
		SetGerritMSFRContainer(&dep)
		r.log.V(1).Info("Gerrit configuration changed, restarting ...")
		// Update the deployment resource
		r.UpdateR(&dep)
	}

	// Create services exposed by Gerrit
	httpd_service := GerritHttpdService(r.ns)
	sshd_service := GerritSshdService(r.ns)
	r.GetOrCreate(&httpd_service)
	r.GetOrCreate(&sshd_service)

	ready := r.IsStatefulSetReady(&dep)
	if ready {
		return r.GerritPostInitJob("post-init")
	} else {
		return false
	}
}

func (r *SFController) setupGerritIngress() {
	r.ensureHTTPSRoute(r.cr.Name+"-gerrit", "gerrit", GERRIT_HTTPD_PORT_NAME, "/", GERRIT_HTTPD_PORT, map[string]string{}, r.cr.Spec.FQDN)
}
