// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the node_exporter setup.
// This is meant as a side-car container for other services
// that do not expose metrics natively for Prometheus.

package controllers

import (
	"math"

	apiv1 "k8s.io/api/core/v1"
)

const NODE_EXPORTER_NAME_SUFFIX = "-nodeexporter"
const NODE_EXPORTER_PORT_NAME_SUFFIX = "-ne"
const NODE_EXPORTER_PORT = 9100

const NODE_EXPORTER_IMAGE = "quay.io/prometheus/node-exporter:latest"

func Get_nodeexporter_port_name(serviceName string) string {
	// Port name is limited to 15 chars
	var length = float64(len(serviceName))
	var upper = int(math.Min(12, length))
	var exporter_port_name = serviceName[:upper] + NODE_EXPORTER_PORT_NAME_SUFFIX
	return exporter_port_name
}

// Fun fact: arrays cannot be consts, so we define our args in this function.
func getNodeExporterArgs(volumeMounts []apiv1.VolumeMount) []string {
	var excludePaths = "^(/etc/hosts|/etc/hostname|/etc/passwd|/etc/resolv.conf|/run/.containerenv|/run/secrets|/dev|/proc|/sys)($|/)"
	return []string{
		"--collector.disable-defaults",
		"--collector.filesystem",
		"--collector.filesystem.mount-points-exclude=" + excludePaths,
	}
}

func createNodeExporterSideCarContainer(serviceName string, volumeMounts []apiv1.VolumeMount) apiv1.Container {

	var exporter_port_name = Get_nodeexporter_port_name(serviceName)

	NODE_EXPORTER_ARGS := getNodeExporterArgs(volumeMounts)
	ports := []apiv1.ContainerPort{
		Create_container_port(NODE_EXPORTER_PORT, exporter_port_name),
	}
	return apiv1.Container{
		Name:            serviceName + NODE_EXPORTER_NAME_SUFFIX,
		Image:           NODE_EXPORTER_IMAGE,
		ImagePullPolicy: "IfNotPresent",
		Args:            NODE_EXPORTER_ARGS,
		VolumeMounts:    volumeMounts,
		Ports:           ports,
		SecurityContext: create_security_context(false),
	}
}

func (r *SFUtilContext) getOrCreateNodeExporterSideCarService(serviceName string) {
	var exporter_port_name = Get_nodeexporter_port_name(serviceName)
	service_ports := []int32{NODE_EXPORTER_PORT}
	ne_service := r.create_service(serviceName+NODE_EXPORTER_PORT_NAME_SUFFIX, serviceName, service_ports, exporter_port_name)
	r.GetOrCreate(&ne_service)
}
