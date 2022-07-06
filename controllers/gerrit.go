// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

package controllers

import (
	_ "embed"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const IDENT = "gerrit"
const GERRIT_HTTPD_PORT = 8080
const GERRIT_HTTPD_PORT_NAME = "gerrit-httpd"
const GERRIT_SSHD_PORT = 29418
const GERRIT_SSHD_PORT_NAME = "gerrit-sshd"
const IMAGE = "quay.io/software-factory/gerrit:3.4.5-2"
const GERRIT_EP_MOUNT_PATH = "/entry"
const GERRIT_GIT_MOUNT_PATH = "/var/gerrit/git"
const GERRIT_INDEX_MOUNT_PATH = "/var/gerrit/index"
const GERRIT_ETC_MOUNT_PATH = "/var/gerrit/etc"
const GERRIT_SSH_MOUNT_PATH = "/var/gerrit/.ssh"
const GERRIT_LOGS_MOUNT_PATH = "/var/gerrit/logs"

//go:embed static/gerrit/post-init.sh
var postInitScript string

//go:embed static/gerrit/entrypoint.sh
var entrypoint string

func (r *SFController) GerritPostInitJob(name string) bool {
	var job batchv1.Job
	job_name := IDENT + "-" + name
	found := r.GetM(job_name, &job)

	if !found {
		// Set post-init.sh in a config map
		cm_data := make(map[string]string)
		cm_data["post-init.sh"] = postInitScript
		r.EnsureConfigMap("gerrit-pi", cm_data)

		container := apiv1.Container{
			Name:    fmt.Sprintf("%s-container", job_name),
			Image:   IMAGE,
			Command: []string{"/bin/bash", "/entry/post-init.sh"},
			Env: []apiv1.EnvVar{
				create_env("FQDN", r.cr.Spec.FQDN),
				create_secret_env("GERRIT_ADMIN_SSH", "gerrit-admin-ssh-key", "priv"),
			},
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      IDENT + "-pi",
					MountPath: "/entry",
				},
			},
		}
		job := create_job(r.ns, job_name, container)
		job.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm(IDENT+"-pi", IDENT+"-pi-config-map"),
		}
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

func (r *SFController) DeployGerrit(enabled bool) bool {
	if enabled {

		// Set entrypoint.sh in a config map
		cm_ep_data := make(map[string]string)
		cm_ep_data["entrypoint.sh"] = entrypoint
		r.EnsureConfigMap("gerrit-ep", cm_ep_data)

		// Set Gerrit env vars in a config map
		cm_config_data := make(map[string]string)
		// Those variables should be populated via the SoftwareFactory CRD Spec
		cm_config_data["SSHD_MAX_CONNECTIONS_PER_USER"] = "20"
		cm_config_data["FQDN"] = r.cr.Spec.FQDN
		r.EnsureConfigMap("gerrit-config", cm_config_data)

		// Ensure Gerrit Admin user ssh key
		r.EnsureSSHKey("gerrit-admin-ssh-key")

		// Create the deployment
		dep := create_statefulset(r.ns, IDENT, IMAGE)
		dep.Spec.Template.Spec.Containers[0].Command = []string{"/bin/bash", "/entry/entrypoint.sh"}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      IDENT + "-ep",
				MountPath: GERRIT_EP_MOUNT_PATH,
			},
			{
				Name:      IDENT + "-git",
				MountPath: GERRIT_GIT_MOUNT_PATH,
			},
			{
				Name:      IDENT + "-index",
				MountPath: GERRIT_INDEX_MOUNT_PATH,
			},
			{
				Name:      IDENT + "-config",
				MountPath: GERRIT_ETC_MOUNT_PATH,
			},
			{
				Name:      IDENT + "-ssh",
				MountPath: GERRIT_SSH_MOUNT_PATH,
			},
			{
				Name:      IDENT + "-logs",
				MountPath: GERRIT_LOGS_MOUNT_PATH,
			},
		}

		dep.Spec.VolumeClaimTemplates = append(
			dep.Spec.VolumeClaimTemplates,
			create_pvc(r.ns, IDENT+"-git"),
			create_pvc(r.ns, IDENT+"-index"),
			create_pvc(r.ns, IDENT+"-config"),
			create_pvc(r.ns, IDENT+"-ssh"),
			create_pvc(r.ns, IDENT+"-logs"),
		)

		// This port definition is informational all ports exposed by the container
		// will be available to the network.
		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME),
			create_container_port(GERRIT_SSHD_PORT, GERRIT_SSHD_PORT_NAME),
		}

		// Expose env vars
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_secret_env("GERRIT_ADMIN_SSH", "gerrit-admin-ssh-key", "priv"),
			create_secret_env("GERRIT_ADMIN_SSH_PUB", "gerrit-admin-ssh-key", "pub"),
		}

		// Expose env vars from a config map
		dep.Spec.Template.Spec.Containers[0].EnvFrom = []apiv1.EnvFromSource{
			{
				ConfigMapRef: &apiv1.ConfigMapEnvSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "gerrit-config-config-map",
					},
				},
			},
		}

		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm(IDENT+"-ep", IDENT+"-ep-config-map"),
		}

		// Create readiness probes
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/", GERRIT_HTTPD_PORT)
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(GERRIT_SSHD_PORT)

		r.Apply(&dep)

		// Create services exposed by Gerrit
		httpd_service := create_service(r.ns, GERRIT_HTTPD_PORT_NAME, IDENT, GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME)
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
					"run": IDENT,
				},
			}}
		r.Apply(&httpd_service)
		r.Apply(&sshd_service)

		// Wait for the service to be ready.
		r.GetM(IDENT, &dep)
		if dep.Status.ReadyReplicas > 0 {
			return r.GerritPostInitJob("post-init")
		} else {
			r.log.V(1).Info("Waiting for Gerrit to be ready...")
			return false
		}
	} else {
		r.DeleteStatefulSet(IDENT)
		r.DeleteService(GERRIT_HTTPD_PORT_NAME)
		r.DeleteService(GERRIT_SSHD_PORT_NAME)
		return true
	}
}

func (r *SFController) IngressGerrit() []netv1.IngressRule {
	return []netv1.IngressRule{
		create_ingress_rule(IDENT+"."+r.cr.Spec.FQDN, GERRIT_HTTPD_PORT_NAME, GERRIT_HTTPD_PORT),
	}
}
