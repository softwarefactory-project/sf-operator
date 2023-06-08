// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

package controllers

import (
	_ "embed"
	"fmt"

	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
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

//go:embed static/gerrit/post-init.sh
var postInitScript string

//go:embed static/gerrit/entrypoint.sh
var entrypoint string

//go:embed static/gerrit/init.sh
var gerritInitScript string

func (r *SFController) GerritInitContainers(volumeMounts []apiv1.VolumeMount) []apiv1.Container {

	container := apiv1.Container{
		Name:    "gerrit-init",
		Image:   GERRIT_IMAGE,
		Command: []string{"sh", "-c", gerritInitScript},
		Env: []apiv1.EnvVar{
			create_secret_env("GERRIT_ADMIN_SSH_PUB", "admin-ssh-key", "pub"),
			create_env("FQDN", r.cr.Spec.FQDN),
		},
		VolumeMounts:    volumeMounts,
		SecurityContext: create_security_context(false),
	}
	return []apiv1.Container{container}
}

func (r *SFController) GerritPostInitJob(name string) bool {
	var job batchv1.Job
	job_name := GERRIT_IDENT + "-" + name
	found := r.GetM(job_name, &job)

	if !found {
		env := []apiv1.EnvVar{
			create_env("HOME", "/tmp"),
			create_env("FQDN", r.cr.Spec.FQDN),
			create_secret_env("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
			create_secret_env("GERRIT_ADMIN_API_KEY", "gerrit-admin-api-key", "gerrit-admin-api-key"),
			create_secret_env("ZUUL_SSH_PUB_KEY", "zuul-ssh-key", "pub"),
			create_secret_env("ZUUL_HTTP_PASSWORD", "zuul-gerrit-api-key", "zuul-gerrit-api-key"),
		}

		container := apiv1.Container{
			Name:            fmt.Sprintf("%s-container", job_name),
			Image:           BUSYBOX_IMAGE,
			Command:         []string{"sh", "-c", postInitScript},
			Env:             env,
			SecurityContext: create_security_context(false),
		}
		job := r.create_job(job_name, container)
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

func (r *SFController) DeployGerrit() bool {

	// Ensure Gerrit Admin API password
	r.GenerateSecretUUID("gerrit-admin-api-key")

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      GERRIT_IDENT,
			MountPath: GERRIT_SITE_MOUNT_PATH,
		},
	}

	// Create the deployment
	replicas := int32(1)
	storage_config := r.baseGetStorageConfOrDefault(v1.StorageSpec{}, "")
	dep := r.create_statefulset(GERRIT_IDENT, GERRIT_IMAGE, storage_config, replicas)

	// Setup the main container
	dep.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", entrypoint}
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME),
		create_container_port(GERRIT_SSHD_PORT, GERRIT_SSHD_PORT_NAME),
	}
	dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		create_env("HOME", "/gerrit"),
		create_env("FQDN", r.cr.Spec.FQDN),
		create_secret_env("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
	}
	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_cmd_probe([]string{"bash", "/gerrit/ready.sh"})
	dep.Spec.Template.Spec.InitContainers = r.GerritInitContainers(volumeMounts)

	// Create annotations based on Gerrit parameters
	annotations := map[string]string{
		"fqdn": r.cr.Spec.FQDN,
	}
	dep.Spec.Template.ObjectMeta.Annotations = annotations
	r.GetOrCreate(&dep)
	if !map_equals(&dep.Spec.Template.ObjectMeta.Annotations, &annotations) {
		// Update the annotation - this force the statefulset controler to respawn the container
		dep.Spec.Template.ObjectMeta.Annotations = annotations
		// ReInit initContainers to ensure new spec is used
		dep.Spec.Template.Spec.InitContainers = r.GerritInitContainers(volumeMounts)
		r.log.V(1).Info("Gerrit configuration changed, restarting ...")
		// Update the deployment resource
		r.UpdateR(&dep)
	}

	// Create services exposed by Gerrit
	httpd_service := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GERRIT_HTTPD_PORT_NAME,
			Namespace: r.ns,
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

	sshd_service := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GERRIT_SSHD_PORT_NAME,
			Namespace: r.ns,
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
