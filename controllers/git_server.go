// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the git-server configuration.

package controllers

import (
	_ "embed"
	"strconv"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const GS_IDENT = "git-server"
const GS_GIT_PORT = 9418
const GS_GIT_PORT_NAME = "git-server-port"
const GS_IMAGE = "quay.io/software-factory/git-deamon:2.39.1-3"
const GS_GIT_MOUNT_PATH = "/git"
const GS_PI_MOUNT_PATH = "/entry"

//go:embed static/git-server/update-system-config.sh
var preInitScript string

func (r *SFController) DeployGitServer() bool {
	cm_data := make(map[string]string)
	cm_data["pre-init.sh"] = preInitScript
	r.EnsureConfigMap(GS_IDENT+"-pi", cm_data)

	annotations := map[string]string{
		"system-config": checksum([]byte(preInitScript)),
	}

	// Create the deployment
	replicas := int32(1)
	dep := r.create_statefulset(GS_IDENT, GS_IMAGE, r.getStorageConfOrDefault(r.cr.Spec.GitServer.Storage), replicas)
	dep.Spec.Template.ObjectMeta.Annotations = annotations
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      GS_IDENT,
			MountPath: GS_GIT_MOUNT_PATH,
		},
	}

	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		create_volume_cm(GS_IDENT+"-pi", GS_IDENT+"-pi-config-map"),
	}

	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(GS_GIT_PORT, GS_GIT_PORT_NAME),
	}

	// Define initContainer
	dep.Spec.Template.Spec.InitContainers = []apiv1.Container{
		{
			Name:            "init-config",
			Image:           GS_IMAGE,
			SecurityContext: create_security_context(false),
			Command:         []string{"/bin/bash", "/entry/pre-init.sh"},
			Env: []apiv1.EnvVar{
				create_env("FQDN", r.cr.Spec.FQDN),
				create_env("LOGSERVER_SSHD_SERVICE_PORT", strconv.Itoa(LOGSERVER_SSHD_PORT)),
			},
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      GS_IDENT,
					MountPath: GS_GIT_MOUNT_PATH,
				},
				{
					Name:      GS_IDENT + "-pi",
					MountPath: GS_PI_MOUNT_PATH,
				},
			},
		},
	}

	// Create readiness probes
	// Note: The probe is causing error message to be logged by the service
	// dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(GS_GIT_PORT)

	r.GetOrCreate(&dep)

	if map_ensure(&dep.Spec.Template.ObjectMeta.Annotations, &annotations) {
		r.log.V(1).Info("System configuration needs to be updated, restarting git-server...")
		r.UpdateR(&dep)
		return false
	}

	// Create services exposed
	git_service := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GS_IDENT,
			Namespace: r.ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:     GS_GIT_PORT_NAME,
					Protocol: apiv1.ProtocolTCP,
					Port:     GS_GIT_PORT,
				},
			},
			Type: apiv1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "sf",
				"run": GS_IDENT,
			},
		}}
	r.GetOrCreate(&git_service)

	return r.IsStatefulSetReady(&dep)
}
