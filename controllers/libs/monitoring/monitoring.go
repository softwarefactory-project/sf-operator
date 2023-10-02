// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

/*
Package monitoring provides various utility functions regarding monitoring for the sf-operator:

* create prometheus monitors and alert rules
* create nodeexporter sidecar
* create statsdexporter sidecar
*/
package monitoring

import (
	"math"
	"strconv"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetTruncatedPortName(serviceName string, suffix string) string {
	// Port name is limited to 15 chars
	var length = float64(len(serviceName))
	var maxChars = 15 - float64(len(suffix))
	var upper = int(math.Min(maxChars, length))
	var exporterPortName = serviceName[:upper] + suffix
	return exporterPortName
}

// Node exporter utilities

const NodeExporterNameSuffix = "-nodeexporter"
const NodeExporterPortNameSuffix = "-ne"
const nodeExporterPort = 9100

const NodeExporterImage = "quay.io/prometheus/node-exporter:latest"

// Fun fact: arrays cannot be consts, so we define our args in this function.
func getNodeExporterArgs(volumeMounts []apiv1.VolumeMount) []string {
	var excludePaths = "^(/etc/hosts|/etc/hostname|/etc/passwd|/etc/resolv.conf|/run/.containerenv|/run/secrets|/dev|/proc|/sys)($|/)"
	return []string{
		"--collector.disable-defaults",
		"--collector.filesystem",
		"--collector.filesystem.mount-points-exclude=" + excludePaths,
	}
}

func MkNodeExporterSideCarContainer(serviceName string, volumeMounts []apiv1.VolumeMount) apiv1.Container {
	container := base.MkContainer(serviceName+NodeExporterNameSuffix, NodeExporterImage)
	container.Args = getNodeExporterArgs(volumeMounts)
	container.Ports = []apiv1.ContainerPort{
		base.MkContainerPort(nodeExporterPort, GetTruncatedPortName(serviceName, NodeExporterPortNameSuffix)),
	}
	container.VolumeMounts = volumeMounts
	return container
}

func MkNodeExporterSideCarService(serviceName string, namespace string) apiv1.Service {
	var portName = GetTruncatedPortName(serviceName, NodeExporterPortNameSuffix)
	servicePorts := []int32{nodeExporterPort}
	neService := base.MkService(serviceName+NodeExporterPortNameSuffix, namespace, serviceName, servicePorts, portName)
	return neService

}

// Statsd exporter utilities

const statsdExporterNameSuffix = "-statsd"
const statsdExporterPortNameSuffix = "-se"
const StatsdExporterPortListen = int32(9125)
const statsdExporterPortExpose = int32(9102)
const StatsdExporterConfigFile = "statsd_mapping.yaml"
const statsdExporterImage = "quay.io/prometheus/statsd-exporter:v0.24.0"

func getStatsdExporterArgs(configPath string, relayAddress *string) []string {
	args := []string{
		"--statsd.mapping-config=" + configPath,
		"--statsd.listen-udp=:" + strconv.Itoa(int(StatsdExporterPortListen)),
		"--web.listen-address=:" + strconv.Itoa(int(statsdExporterPortExpose)),
	}
	if relayAddress != nil {
		args = append(args, "--statsd.relay.address="+*relayAddress)
	}
	return args
}

func GetStatsdExporterPort(serviceName string) string {
	return GetTruncatedPortName(serviceName, statsdExporterPortNameSuffix+"e")
}

func MkStatsdExporterSideCarContainer(serviceName string, configVolumeName string, relayAddress *string) apiv1.Container {
	var seListenPortName = GetTruncatedPortName(serviceName, statsdExporterPortNameSuffix+"l")
	var seExposePortName = GetStatsdExporterPort(serviceName)
	var configFile = StatsdExporterConfigFile
	var configPath = "/tmp/" + configFile
	// var configVolumeName = serviceName + "-statsd-conf"

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      configVolumeName,
			MountPath: configPath,
			SubPath:   configFile,
		},
	}
	args := getStatsdExporterArgs(configPath, relayAddress)
	ports := []apiv1.ContainerPort{
		{
			Name:          seListenPortName,
			Protocol:      apiv1.ProtocolUDP,
			ContainerPort: StatsdExporterPortListen,
		},
		{
			Name:          seExposePortName,
			Protocol:      apiv1.ProtocolTCP,
			ContainerPort: statsdExporterPortExpose,
		},
	}
	sidecar := base.MkContainer(serviceName+statsdExporterNameSuffix, statsdExporterImage)
	sidecar.Args = args
	sidecar.VolumeMounts = volumeMounts
	sidecar.Ports = ports

	return sidecar
}

// Prometheus utilities

// ServiceMonitorLabelSelector - TODO this could be a spec parameter.
const ServiceMonitorLabelSelector = "sf-monitoring"

func MkPrometheusRuleGroup(name string, rules []monitoringv1.Rule) monitoringv1.RuleGroup {
	// d := monitoringv1.Duration(duration)
	return monitoringv1.RuleGroup{
		Name: name,
		// Interval: &d,
		Rules: rules,
	}
}

var CriticalSeverityLabel = map[string]string{
	"severity": "critical",
}

var WarningSeverityLabel = map[string]string{
	"severity": "warning",
}

func MkPrometheusAlertRule(name string, expr intstr.IntOrString, forDuration string, labels map[string]string, annotations map[string]string) monitoringv1.Rule {
	f := monitoringv1.Duration(forDuration)
	return monitoringv1.Rule{
		Alert:       name,
		Expr:        expr,
		For:         &f,
		Labels:      labels,
		Annotations: annotations,
	}
}

//lint:ignore U1000 this function will be used in a followup change
func mkServiceMonitor(name string, ns string, portName string, selector metav1.LabelSelector) monitoringv1.ServiceMonitor {
	return monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				ServiceMonitorLabelSelector: name,
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: monitoringv1.Duration("30s"),
					Port:     portName,
					Scheme:   "http",
				},
			},
			Selector: selector,
		},
	}
}

func MkPodMonitor(name string, ns string, ports []string, selector metav1.LabelSelector) monitoringv1.PodMonitor {
	endpoints := []monitoringv1.PodMetricsEndpoint{}
	for _, port := range ports {
		endpoints = append(endpoints, monitoringv1.PodMetricsEndpoint{Port: port})
	}

	return monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				ServiceMonitorLabelSelector: name,
			},
		},
		Spec: monitoringv1.PodMonitorSpec{
			Selector:            selector,
			PodMetricsEndpoints: endpoints,
		},
	}
}

func MkPrometheusRuleCR(name string, ns string) monitoringv1.PrometheusRule {
	return monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				ServiceMonitorLabelSelector: name,
			},
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{},
		},
	}
}
