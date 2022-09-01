// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the etherpad configuration.
package controllers

import (
	_ "embed"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const MOSQUITTO_IDENT string = "mosquitto"
const MOSQUITTO_IMAGE string = "quay.io/software-factory/mosquitto:2.0.14-1"

// //go:embed static/mosquitto/cmdProbe.sh
// var mosquittoCmdProbe string

//go:embed static/mosquitto/entrypoint.sh
var mosquittoEntrypointScript string

const MOSQUITTO_PORT_LISTENER_1 = 1883
const MOSQUITTO_PORT_NAME_LISTENER_1 = "mosquittoport1"
const MOSQUITTO_PORT_LISTENER_2 = 1884
const MOSQUITTO_PORT_NAME_LISTENER_2 = "mosquittoport2"

func (r *SFController) DeployMosquitto(spec sfv1.BaseSpec) bool {

	if spec.Enabled {

		dep := create_deployment(r.ns, MOSQUITTO_IDENT, MOSQUITTO_IMAGE)
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"sh", "-c", mosquittoEntrypointScript}

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(MOSQUITTO_PORT_LISTENER_1, MOSQUITTO_PORT_NAME_LISTENER_1),
			create_container_port(MOSQUITTO_PORT_LISTENER_2, MOSQUITTO_PORT_NAME_LISTENER_2),
		}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      MOSQUITTO_IDENT + "-config-vol",
				MountPath: "/etc/mosquitto",
			},
		}

		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_empty_dir(MOSQUITTO_IDENT + "-config-vol"),
		}

		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_cmd_probe([]string{
			"timeout",
			"--preserve-status",
			"2",
			"mosquitto_sub",
			"-t",
			"#",
		})

		r.GetOrCreate(&dep)

		srv_listener_1 := create_service(r.ns, MOSQUITTO_IDENT, MOSQUITTO_IDENT, MOSQUITTO_PORT_LISTENER_1, MOSQUITTO_PORT_NAME_LISTENER_1)
		r.GetOrCreate(&srv_listener_1)

		srv_listener_2 := create_service(r.ns, MOSQUITTO_IDENT+"-websocket", MOSQUITTO_IDENT+"-websocket", MOSQUITTO_PORT_LISTENER_2, MOSQUITTO_PORT_NAME_LISTENER_2)
		r.GetOrCreate(&srv_listener_2)

		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment(MOSQUITTO_IDENT)
		r.DeleteService(MOSQUITTO_PORT_NAME_LISTENER_1)
		r.DeleteService(MOSQUITTO_PORT_NAME_LISTENER_2)
		r.DeleteSecret("mosquitto-session-key")
		r.DeleteSecret("mosquitto-db-password")
		r.DeleteConfigMap("mosquitto-umosquittod-config-map")
		r.DeleteConfigMap("mosquitto-probe-config-map")
		return true
	}
}

func (r *SFController) IngressMosquitto() []netv1.IngressRule {
	return []netv1.IngressRule{
		create_ingress_rule(MOSQUITTO_IDENT+"."+r.cr.Spec.FQDN, MOSQUITTO_IDENT, MOSQUITTO_PORT_LISTENER_1),
		create_ingress_rule(MOSQUITTO_IDENT+"-websocket"+"."+r.cr.Spec.FQDN, MOSQUITTO_IDENT+"-websocket", MOSQUITTO_PORT_LISTENER_2),
	}
}
