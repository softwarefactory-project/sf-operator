// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the git-server configuration.

package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const GS_IDENT = "git-server"
const GS_GIT_PORT = 9418
const GS_GIT_PORT_NAME = "git-server-port"
const GS_IMAGE = "quay.io/software-factory/git-deamon:1.0-1"
const GS_GIT_MOUNT_PATH = "/git"

func (r *SFController) DeployGitServer(enabled bool) bool {
	if enabled {

		// Create the deployment
		dep := create_statefulset(r.ns, GS_IDENT, GS_IMAGE)
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      GS_IDENT + "-git",
				MountPath: GS_GIT_MOUNT_PATH,
			},
		}

		dep.Spec.VolumeClaimTemplates = append(
			dep.Spec.VolumeClaimTemplates,
			create_pvc(r.ns, GS_IDENT+"-git"),
		)

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(GS_GIT_PORT, GS_GIT_PORT_NAME),
		}

		// Define initContainer
		dep.Spec.Template.Spec.InitContainers = []apiv1.Container{
			{
				Name:  "init-config",
				Image: GS_IMAGE,
				Command: []string{
					"/usr/bin/git", "init", "--bare", "/git/system-config"},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      GS_IDENT + "-git",
						MountPath: GS_GIT_MOUNT_PATH,
					},
				},
			},
		}

		// Create readiness probes
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(GS_GIT_PORT)

		r.Apply(&dep)

		// Create services exposed
		git_service := apiv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GS_GIT_PORT_NAME,
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
		r.Apply(&git_service)

		return r.IsStatefulSetReady(GS_IDENT)
	} else {
		r.DeleteStatefulSet(GS_IDENT)
		r.DeleteService(GS_GIT_PORT_NAME)
		return true
	}
}
