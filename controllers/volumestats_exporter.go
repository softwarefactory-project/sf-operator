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

const nameSuffix = "-nodeexporter"
const portNameSuffix = "-ne"
const port = 9100

const NodeExporterImage = "quay.io/prometheus/node-exporter:latest"

func GetNodeexporterPortName(serviceName string) string {
	// Port name is limited to 15 chars
	var length = float64(len(serviceName))
	var upper = int(math.Min(12, length))
	var exporterPortName = serviceName[:upper] + portNameSuffix
	return exporterPortName
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

	var exporterPortName = GetNodeexporterPortName(serviceName)

	exporterArgs := getNodeExporterArgs(volumeMounts)
	ports := []apiv1.ContainerPort{
		MKContainerPort(port, exporterPortName),
	}
	return apiv1.Container{
		Name:            serviceName + nameSuffix,
		Image:           NodeExporterImage,
		ImagePullPolicy: "IfNotPresent",
		Args:            exporterArgs,
		VolumeMounts:    volumeMounts,
		Ports:           ports,
		SecurityContext: mkSecurityContext(false),
	}
}

func (r *SFUtilContext) getOrCreateNodeExporterSideCarService(serviceName string) {
	var portName = GetNodeexporterPortName(serviceName)
	servicePorts := []int32{port}
	neService := r.mkService(serviceName+portNameSuffix, serviceName, servicePorts, portName)
	r.GetOrCreate(&neService)
}
