// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the etherpad configuration.
package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

//go:embed static/etherpad/settings.json
var etherpadSettings string

const ETHERPAD_PORT = 8080
const ETHERPAD_PORT_NAME = "etherpad-port"

func (r *SFController) DeployEtherpad(enabled bool) bool {
	if enabled {
		initContainers, _ := r.EnsureDBInit("etherpad")
		r.EnsureSecret("etherpad-admin-password")
		cm_data := make(map[string]string)
		cm_data["settings.json"] = etherpadSettings
		r.EnsureConfigMap("etherpad", cm_data)

		dep := create_deployment(r.ns, "etherpad", "quay.io/software-factory/sf-etherpad:1.8.17-1")
		dep.Spec.Template.Spec.InitContainers = initContainers
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"node", "./src/node/server.js", "--settings", "/etc/etherpad/settings.json"}
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_secret_env("DB_PASS", "etherpad-db-password", "etherpad-db-password"),
			create_secret_env("ADMIN_PASS", "etherpad-admin-password", "etherpad-admin-password"),
		}
		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(ETHERPAD_PORT, ETHERPAD_PORT_NAME),
		}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      "config-volume",
				MountPath: "/etc/etherpad",
			},
		}
		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm("config-volume", "etherpad-config-map"),
		}
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/api", 8080)
		r.Apply(&dep)
		srv := create_service(r.ns, "etherpad", "etherpad", ETHERPAD_PORT, ETHERPAD_PORT_NAME)
		r.Apply(&srv)

		return r.IsDeploymentReady("etherpad")
	} else {
		r.DeleteDeployment("etherpad")
		r.DeleteService("etherpad")
		return true
	}
}

func (r *SFController) IngressEtherpad() netv1.IngressRule {
	return create_ingress_rule("etherpad."+r.cr.Spec.FQDN, "etherpad", ETHERPAD_PORT)
}
