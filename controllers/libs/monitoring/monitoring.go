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

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"golang.org/x/exp/maps"
	apiv1 "k8s.io/api/core/v1"
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

func MkNodeExporterSideCarContainer(serviceName string, volumeMounts []apiv1.VolumeMount, openshiftUser bool) apiv1.Container {
	container := base.MkContainer(serviceName+NodeExporterNameSuffix, base.NodeExporterImage(), openshiftUser)
	container.Args = getNodeExporterArgs(volumeMounts)
	container.Ports = []apiv1.ContainerPort{
		base.MkContainerPort(NodeExporterPort, GetTruncatedPortName(serviceName, NodeExporterPortNameSuffix)),
	}
	container.VolumeMounts = volumeMounts
	base.SetContainerLimitsLowProfile(&container)
	return container
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

func MkStatsdExporterSideCarContainer(serviceName string, configVolumeName string, relayAddress *string, openshiftUser bool) apiv1.Container {
	var seListenPortName = GetTruncatedPortName(serviceName, statsdExporterPortNameSuffix+"l")
	var seExposePortName = GetStatsdExporterPort(serviceName)
	var configFile = StatsdExporterConfigFile
	var configPath = "/tmp/" + configFile

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
	sidecar := base.MkContainer(serviceName+statsdExporterNameSuffix, base.StatsdExporterImage(), openshiftUser)
	sidecar.Args = args
	sidecar.VolumeMounts = volumeMounts
	sidecar.Ports = ports
	return sidecar
}

// Prometheus utilities

// ServiceMonitorLabelSelector - TODO this could be a spec parameter.
const ServiceMonitorLabelSelector = "sf-monitoring"

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
		cloudNames := maps.Keys(cCs)
		sort.Strings(cloudNames)
		for _, cloudName := range cloudNames {
			cloudConfig := cCs[cloudName]
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
