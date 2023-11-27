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
	"sort"
	"strconv"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
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
const NodeExporterPort = 9100

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
	container := base.MkContainer(serviceName+NodeExporterNameSuffix, base.NodeExporterImage)
	container.Args = getNodeExporterArgs(volumeMounts)
	container.Ports = []apiv1.ContainerPort{
		base.MkContainerPort(NodeExporterPort, GetTruncatedPortName(serviceName, NodeExporterPortNameSuffix)),
	}
	container.VolumeMounts = volumeMounts
	return container
}

func MkDiskUsageRuleGroup(ns string, componentIdent string) monitoringv1.RuleGroup {
	// Create some default, interesting alerts
	diskFullAnnotations := map[string]string{
		"description": "Mountpoint {{ $labels.mountpoint }} on pod {{ $labels.pod }} has {{ $value | humanize1024 }}% free space left.",
		"summary":     "{{ $labels.pod }}:{{ $labels.mountpoint }} out of disk",
	}
	diskFull3daysAnnotations := map[string]string{
		"description": "Mountpoint {{ $labels.mountpoint }} on pod {{ $labels.pod }} has {{ $value | humanize1024 }}% free space left and is expected to fill up in less than three days.",
		"summary":     "{{ $labels.pod }}:{{ $labels.mountpoint }} running out of disk",
	}
	diskFull := MkPrometheusAlertRule(
		"OutOfDiskNow",
		intstr.FromString(
			"(node_filesystem_avail_bytes{job=\""+ns+"/"+componentIdent+"-monitor\"} * 100 /"+
				" node_filesystem_size_bytes{job=\""+ns+"/"+componentIdent+"-monitor\"} < 10) and "+
				"(node_filesystem_avail_bytes{job=\""+ns+"/"+componentIdent+"-monitor\"} < 20 * 1024 ^ 3)"),
		"30m",
		CriticalSeverityLabel,
		diskFullAnnotations,
	)
	diskFullIn3days := MkPrometheusAlertRule(
		"OutOfDiskInThreeDays",
		intstr.FromString(
			"(node_filesystem_avail_bytes{job=\""+ns+"/"+componentIdent+"-monitor\"} * 100 /"+
				" node_filesystem_size_bytes{job=\""+ns+"/"+componentIdent+"-monitor\"} < 50) and "+
				"(predict_linear(node_filesystem_avail_bytes{job=\""+ns+"/"+componentIdent+"-monitor\"}[1d], 3 * 24 * 3600) < 0) and "+
				"(node_filesystem_size_bytes{job=\""+ns+"/"+componentIdent+"-monitor\"} <= 1e+11)"),
		"12h",
		WarningSeverityLabel,
		diskFull3daysAnnotations,
	)
	DiskRuleGroup := MkPrometheusRuleGroup(
		componentIdent+"-disk-usage-default.rules",
		[]monitoringv1.Rule{diskFull, diskFullIn3days})

	return DiskRuleGroup
}

func MkDiskUsagePromRule(ruleGroups []monitoringv1.RuleGroup, ns string) monitoringv1.PrometheusRule {
	desiredDUPromRule := MkPrometheusRuleCR("disk-usage-default.rules", ns)
	desiredDUPromRule.Spec.Groups = append(desiredDUPromRule.Spec.Groups, ruleGroups...)

	var checksumable string
	for _, group := range desiredDUPromRule.Spec.Groups {
		for _, rule := range group.Rules {
			checksumable += MkAlertRuleChecksumString(rule)
		}
	}

	// add annotations so we can handle lifecycle
	annotations := map[string]string{
		"version":       "1",
		"rulesChecksum": utils.Checksum([]byte(checksumable)),
	}

	desiredDUPromRule.ObjectMeta.Annotations = annotations
	return desiredDUPromRule
}

// Statsd exporter utilities

const statsdExporterNameSuffix = "-statsd"
const statsdExporterPortNameSuffix = "-se"
const StatsdExporterPortListen = int32(9125)
const statsdExporterPortExpose = int32(9102)
const StatsdExporterConfigFile = "statsd_mapping.yaml"

type StatsdMetricMappingLabel struct {
	LabelName  string
	LabelValue string
}

type StatsdMetricMapping struct {
	Name         string
	ProviderName string
	Match        string
	Help         string
	Labels       []StatsdMetricMappingLabel
}

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
	sidecar := base.MkContainer(serviceName+statsdExporterNameSuffix, base.StatsdExporterImage)
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

func MkAlertRuleChecksumString(alertRule monitoringv1.Rule) string {
	var checksumable string
	checksumable += alertRule.Alert
	checksumable += alertRule.Expr.String()
	if alertRule.For != nil {
		_for := *alertRule.For
		checksumable += string(_for)
	}

	var labels []string
	for label := range alertRule.Labels {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		checksumable += label + "." + alertRule.Labels[label]
	}

	var annotations []string
	for annotation := range alertRule.Annotations {
		annotations = append(annotations, annotation)
	}
	sort.Strings(annotations)
	for _, annotation := range annotations {
		checksumable += annotation + ":" + alertRule.Annotations[annotation]
	}
	return checksumable
}

func MkStatsdMappingsFromCloudsYaml(extraMappings []StatsdMetricMapping, cloudsYaml map[string]interface{}) []StatsdMetricMapping {
	// Default prefix used by openstacksdk if not set in clouds.yaml
	var globalPrefix = "openstack.api"

	// Parse clouds.yaml for statsd prefixes
	if globalMetricsConf, ok := cloudsYaml["metrics"]; ok {
		gmc := globalMetricsConf.(map[string]interface{})
		if globalStatsdConf, ok := gmc["statsd"]; ok {
			gsc := globalStatsdConf.(map[string]interface{})
			if prefix, ok := gsc["prefix"]; ok {
				globalPrefix = prefix.(string)
			}
		}
	}
	if cloudConfigs, ok := cloudsYaml["clouds"]; ok {
		cCs := cloudConfigs.(map[string]interface{})
		for cloudName, cloudConfig := range cCs {
			cC := cloudConfig.(map[string]interface{})
			if metricsConf, ok := cC["metrics"]; ok {
				mC := metricsConf.(map[string]interface{})
				if statsdConf, ok := mC["statsd"]; ok {
					sC := statsdConf.(map[string]interface{})
					if prefx, ok := sC["prefix"]; ok {
						prefix := prefx.(string)
						var extraMapping = StatsdMetricMapping{
							Name:         strings.Replace(prefix, ".", "_", -1),
							ProviderName: cloudName,
							Match:        prefix + ".*.*.*.*",
							Help:         "API calls metrics issued by openstacksdk for cloud " + cloudName,
							Labels: []StatsdMetricMappingLabel{
								{LabelName: "service", LabelValue: "$1"},
								{LabelName: "method", LabelValue: "$2"},
								{LabelName: "resource", LabelValue: "$3"},
								{LabelName: "status", LabelValue: "$4"},
							},
						}
						extraMappings = append(extraMappings, extraMapping)
					}
				}
			}
		}
	}

	// Add default openstacksdk metric emission
	extraMappings = append(extraMappings, StatsdMetricMapping{
		Name:         strings.Replace(globalPrefix, ".", "_", -1),
		ProviderName: "ALL",
		Match:        globalPrefix + ".*.*.*.*",
		Help:         "API calls metrics issued by openstacksdk",
		Labels: []StatsdMetricMappingLabel{
			{LabelName: "service", LabelValue: "$1"},
			{LabelName: "method", LabelValue: "$2"},
			{LabelName: "resource", LabelValue: "$3"},
			{LabelName: "status", LabelValue: "$4"},
		},
	})
	return extraMappings
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
