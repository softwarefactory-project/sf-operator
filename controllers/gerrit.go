// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

package controllers

import (
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const IDENT = "gerrit"
const GERRIT_HTTPD_PORT = 8080
const GERRIT_HTTPD_PORT_NAME = "gerrit-httpd"
const GERRIT_SSHD_PORT = 29418
const GERRIT_SSHD_PORT_NAME = "gerrit-sshd"
const IMAGE = "quay.io/software-factory/gerrit:3.4.5-1"
const JAVA_OPTIONS = "-Djava.security.egd=file:/dev/./urandom"
const GERRIT_EP_MOUNT_PATH = "/entry"
const GERRIT_GIT_MOUNT_PATH = "/var/gerrit/git"
const GERRIT_INDEX_MOUNT_PATH = "/var/gerrit/index"
const GERRIT_ETC_MOUNT_PATH = "/var/gerrit/etc"

// TODO: Attach a config-map for all Gerrit option (like sshd.maxConnectionsPerUser) then
// make entrypoint.sh uses ENV vars exposed by the configMap
const GERRIT_ENTRYPOINT = `
#!/bin/bash

# The /dev/./urandom is not a typo. https://stackoverflow.com/questions/58991966/what-java-security-egd-option-is-for
JAVA_OPTIONS="-Djava.security.egd=file:/dev/./urandom"

echo "Initializing Gerrit site ..."
java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war init -d /var/gerrit --batch --no-auto-start --skip-plugins
java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war reindex -d /var/gerrit

echo "Setting Gerrit config file ..."
git config -f /var/gerrit/etc/gerrit.config --replace-all auth.type "DEVELOPMENT_BECOME_ANY_ACCOUNT"
git config -f /var/gerrit/etc/gerrit.config --replace-all sshd.listenaddress "*:29418"
git config -f /var/gerrit/etc/gerrit.config --replace-all sshd.idleTimeout "2d"
git config -f /var/gerrit/etc/gerrit.config --replace-all sshd.maxConnectionsPerUser "${SSHD_MAX_CONNECTIONS_PER_USER:-10}"

echo "Running Gerrit ..."
exec java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war daemon -d /var/gerrit
`

func (r *SFController) DeployGerrit(enabled bool) bool {
	var dep appsv1.StatefulSet
	found := r.GetM(IDENT, &dep)
	if enabled && !found {
		r.log.V(1).Info("Deploying " + IDENT)

		// Set entrypoint.sh in a config map
		cm_ep_data := make(map[string]string)
		cm_ep_data["entrypoint.sh"] = GERRIT_ENTRYPOINT
		r.EnsureConfigMap("gerrit-ep", cm_ep_data)

		// Set Gerrit env vars in a config map
		cm_config_data := make(map[string]string)
		// Those variables should be populated via the SoftwareFactory CRD Spec
		cm_config_data["SSHD_MAX_CONNECTIONS_PER_USER"] = "20"
		r.EnsureConfigMap("gerrit-config", cm_config_data)

		// Ensure Gerrit Admin user ssh key
		r.EnsureSSHKey("gerrit-admin-ssh-key")

		// Create the deployment
		dep = create_statefulset(r.ns, IDENT, IMAGE)
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
		}

		dep.Spec.VolumeClaimTemplates = append(
			dep.Spec.VolumeClaimTemplates,
			create_pvc(r.ns, IDENT+"-git"),
			create_pvc(r.ns, IDENT+"-index"),
			create_pvc(r.ns, IDENT+"-config"))

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
		r.CreateR(&dep)

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
		r.CreateR(&httpd_service)
		r.CreateR(&sshd_service)
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

func (r *SFController) IngressGerrit() []netv1.IngressRule {
	return []netv1.IngressRule{
		create_ingress_rule(IDENT+"."+r.cr.Spec.FQDN, GERRIT_HTTPD_PORT_NAME, GERRIT_HTTPD_PORT),
		// How to expose no HTTP traffic ?
		// create_ingress_rule(GERRIT_SSHD_PORT_NAME+r.cr.Spec.FQDN, GERRIT_SSHD_PORT_NAME, GERRIT_SSHD_PORT),
	}
}
