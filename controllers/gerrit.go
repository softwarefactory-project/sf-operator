// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

package controllers

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

const IDENT = "gerrit"
const GERRIT_PORT = 8080
const GERRIT_PORT_NAME = "gerrit-port"
const IMAGE = "quay.io/software-factory/gerrit:3.4.3-0"
const JAVA_OPTIONS = "-Djava.security.egd=file:/dev/./urandom"
const GERRIT_GIT_MOUNT_PATH = "/var/gerrit/git"
const GERRIT_INDEX_MOUNT_PATH = "/var/gerrit/index"

var entrypoint = []string{"/bin/bash", "-c",
	fmt.Sprintf("/usr/bin/java %s -jar /var/gerrit/bin/gerrit.war init -d /var/gerrit --batch --no-auto-start --skip-plugins", JAVA_OPTIONS) +
		" && " + fmt.Sprintf("/usr/bin/java %s -jar /var/gerrit/bin/gerrit.war reindex -d /var/gerrit", JAVA_OPTIONS) +
		" && " + fmt.Sprintf("/usr/bin/java %s -jar /var/gerrit/bin/gerrit.war daemon -d /var/gerrit", JAVA_OPTIONS)}

func (r *SFController) DeployGerrit(enabled bool) bool {
	var dep appsv1.StatefulSet
	found := r.GetM(IDENT, &dep)
	if enabled && !found {
		r.log.V(1).Info("Deploying " + IDENT)
		dep = create_statefulset(r.ns, IDENT, IMAGE)
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      IDENT + "-git",
				MountPath: GERRIT_GIT_MOUNT_PATH,
			},
			{
				Name:      IDENT + "-index",
				MountPath: GERRIT_INDEX_MOUNT_PATH,
			},
		}
		// This port defintion is informational all ports exposed by the container
		// will be available to the network.
		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(GERRIT_PORT, GERRIT_PORT_NAME),
		}
		// r.log.V(1).Info(entrypoint)
		dep.Spec.Template.Spec.Containers[0].Command = entrypoint
		// dep.Spec.Template.Spec.RestartPolicy = apiv1.RestartPolicyNever
		r.CreateR(&dep)
		srv := create_service(r.ns, IDENT, GERRIT_PORT, GERRIT_PORT_NAME)
		r.CreateR(&srv)
	}
	if !enabled && found {
		r.log.V(1).Info("Destroying " + IDENT)
		if err := r.Delete(r.ctx, &dep); err != nil {
			panic(err.Error())
		}
	}
	if enabled && found {
		// Wait for the service to be ready.
		return (dep.Status.ReadyReplicas > 0)
	}
	return true
}
