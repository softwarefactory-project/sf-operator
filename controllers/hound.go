// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the hound configuration.
package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const HOUND_IDENT string = "hound"
const HOUND_IMAGE string = "quay.io/software-factory/hound:0.5.1-1"

const HOUND_PORT = 6080
const HOUND_PORT_NAME = "hound-port"

func GenerateHoundConfig() string {
	return "{\n" +
		"    \"max-concurrent-indexers\": 2,\n" +
		"    \"dbpath\": \"/var/lib/hound/data\",\n" +
		"    \"repos\": {}\n" +
		"}\n"
}

func (r *SFController) DeployHound(enabled bool) bool {

	if enabled {
		// Creating hound config.json file
		config_json := make(map[string]string)
		config_json["config.json"] = GenerateHoundConfig()
		r.EnsureConfigMap(HOUND_IDENT, config_json)

		dep := create_deployment(r.ns, HOUND_IDENT, HOUND_IMAGE)
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"/go/bin/houndd", "-conf", "/data/config.json"}

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(HOUND_PORT, HOUND_PORT_NAME),
		}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      HOUND_IDENT + "-config-vol",
				MountPath: "/data",
			},
			{
				Name:      HOUND_IDENT + "-repos-vol",
				MountPath: "/var/lib/hound",
			},
		}

		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm(HOUND_IDENT+"-config-vol", HOUND_IDENT+"-config-map"),
			create_empty_dir(HOUND_IDENT + "-repos-vol"),
		}

		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(HOUND_PORT)

		r.GetOrCreate(&dep)

		srv := create_service(r.ns, HOUND_IDENT, HOUND_IDENT, HOUND_PORT, HOUND_PORT_NAME)
		r.GetOrCreate(&srv)

		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment(HOUND_IDENT)
		r.DeleteService(HOUND_PORT_NAME)
		r.DeleteConfigMap("hound-config-map")
		return true
	}
}

func (r *SFController) IngressHound() netv1.IngressRule {
	return create_ingress_rule(HOUND_IDENT+"."+r.cr.Spec.FQDN, HOUND_IDENT, HOUND_PORT)
}
