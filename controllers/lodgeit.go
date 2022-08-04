// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the etherpad configuration.
package controllers

import (
	_ "embed"
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const LODGEIT_IDENT string = "lodgeit"
const LODGEIT_IMAGE string = "quay.io/software-factory/lodgeit:0.3-1"

//go:embed static/lodgeit/entrypoint.sh
var lodgeitEntrypointScript string

const LODGEIT_PORT = 5000
const LODGEIT_PORT_NAME = "lodgeit"

func (r *SFController) DeployLodgeit(enabled bool) bool {
	if enabled {
		initContainers, _ := r.EnsureDBInit("lodgeit")

		// Generating Lodgeit Passwords
		r.GenerateSecretUUID("lodgeit-session-key")

		dep := create_deployment(r.ns, LODGEIT_IDENT, LODGEIT_IMAGE)
		dep.Spec.Template.Spec.InitContainers = initContainers
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"sh", "-c", lodgeitEntrypointScript}
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_secret_env("LODGEIT_MYSQL_PASSWORD", "lodgeit-db-password", "lodgeit-db-password"),
			create_secret_env("LODGEIT_SESSION_KEY", "lodgeit-session-key",
				"lodgeit-session-key"),
		}

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(LODGEIT_PORT, LODGEIT_PORT_NAME),
		}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      LODGEIT_IDENT + "-config-vol",
				MountPath: "/etc/lodgeit",
			},
		}

		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_empty_dir(LODGEIT_IDENT + "-config-vol"),
		}

		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/", LODGEIT_PORT)

		r.GetOrCreate(&dep)
		srv := create_service(r.ns, LODGEIT_IDENT, LODGEIT_IDENT, LODGEIT_PORT, LODGEIT_PORT_NAME)
		r.GetOrCreate(&srv)

		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment(LODGEIT_IDENT)
		r.DeleteService(LODGEIT_PORT_NAME)
		r.DeleteSecret("lodgeit-session-key")
		r.DeleteSecret("lodgeit-db-password")
		r.DeleteConfigMap("lodgeit-ep-config-map")
		return true
	}
}

func (r *SFController) IngressLodgeit() netv1.IngressRule {
	fmt.Println(LODGEIT_IDENT + "." + r.cr.Spec.FQDN)
	return create_ingress_rule(LODGEIT_IDENT+"."+r.cr.Spec.FQDN, LODGEIT_PORT_NAME, LODGEIT_PORT)
}
