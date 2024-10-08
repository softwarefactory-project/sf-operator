// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

/*
Package logging provides various utility functions regarding
optional service log collection for the sf-operator:

* create fluent bit sidecar
*/
package logging

import (
	_ "embed"
	"strconv"

	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	apiv1 "k8s.io/api/core/v1"
)

type FluentBitLabel struct {
	Key   string
	Value string
}

type TemplateInputParams struct {
	InUse bool
	Host  string
	Port  string
}

type TemplateLoggingParams struct {
	Tag                string
	LogLevel           string
	HTTPInputConfig    TemplateInputParams
	ForwardInputConfig TemplateInputParams
}

var (
	//go:embed static/sfExtras.py
	SFExtrasPythonModule string
)

func CreateForwarderEnvVars(name string, extraLabels []FluentBitLabel) []apiv1.EnvVar {
	forwarderEnvVars := []apiv1.EnvVar{
		base.MkEnvVarFromFieldRef("K8S_NODENAME", "spec.nodeName"),
		base.MkEnvVarFromFieldRef("K8S_PODNAME", "metadata.name"),
		base.MkEnvVarFromFieldRef("K8S_NAMESPACE", "metadata.namespace"),
		base.MkEnvVarFromFieldRef("K8S_PODIP", "status.podIP"),
		base.MkEnvVar("K8S_LABELS_RUN", name),
		base.MkEnvVar("K8S_LABELS_APP", "sf"),
	}
	for i := range extraLabels {
		var v = base.MkEnvVar("K8S_"+extraLabels[i].Key, extraLabels[i].Value)
		forwarderEnvVars = append(forwarderEnvVars, v)
	}
	return forwarderEnvVars
}

func CreateBaseLoggingExtraKeys(name string, component string, container string, namespace string) []FluentBitLabel {
	baseExtraKeys := []FluentBitLabel{
		{
			Key:   "labels_app",
			Value: "sf",
		},
		{
			Key:   "labels_run",
			Value: name,
		},
		{
			Key:   "component",
			Value: component,
		},
		{
			Key:   "namespace",
			Value: namespace,
		},
		{
			Key:   "container",
			Value: container,
		},
	}
	return baseExtraKeys
}

func CreateForwarderConfigTemplateParams(tag string, forwarderSpec *v1.FluentBitForwarderSpec) TemplateLoggingParams {
	var httpInputParams = TemplateInputParams{
		InUse: false,
		Host:  "",
		Port:  "",
	}
	var forwardInputParams = TemplateInputParams{
		InUse: false,
		Host:  "",
		Port:  "",
	}
	var loggingParams = TemplateLoggingParams{
		Tag:                tag,
		LogLevel:           "info",
		HTTPInputConfig:    httpInputParams,
		ForwardInputConfig: forwardInputParams,
	}
	if forwarderSpec != nil {
		if forwarderSpec.HTTPInputHost != "" {
			loggingParams.HTTPInputConfig.InUse = true
			loggingParams.HTTPInputConfig.Host = forwarderSpec.HTTPInputHost
			loggingParams.HTTPInputConfig.Port = strconv.Itoa(int(forwarderSpec.HTTPInputPort))
		}
		if forwarderSpec.ForwardInputHost != "" {
			loggingParams.ForwardInputConfig.InUse = true
			loggingParams.ForwardInputConfig.Host = forwarderSpec.ForwardInputHost
			loggingParams.ForwardInputConfig.Port = strconv.Itoa(int(forwarderSpec.ForwardInputPort))
		}

		if forwarderSpec.Debug != nil && *forwarderSpec.Debug {
			loggingParams.LogLevel = "debug"
		}
	}
	return loggingParams
}

func SetupLogForwarding(serviceName string, forwarderSpec *v1.FluentBitForwarderSpec, extraLabels []FluentBitLabel, annotations map[string]string) []apiv1.EnvVar {
	if forwarderSpec != nil {
		annotations["log-forwarding"] = forwarderSpec.HTTPInputHost + ":" + strconv.Itoa(int(forwarderSpec.HTTPInputPort))
		annotations["log-forwarding"] += forwarderSpec.ForwardInputHost + ":" + strconv.Itoa(int(forwarderSpec.ForwardInputPort))
		return CreateForwarderEnvVars(serviceName, extraLabels)
	} else {
		annotations["log-forwarding"] = "disabled"
		return []apiv1.EnvVar{}
	}
}

func CreateFluentBitSideCarContainer(serviceName string, extraLabels []FluentBitLabel, volumeMounts []apiv1.VolumeMount, debug bool) (apiv1.Container, apiv1.Volume) {
	var img = base.FluentBitImage(debug)
	container := base.MkContainer("fluentbit", img)
	container.Env = CreateForwarderEnvVars(serviceName, extraLabels)
	ports := []apiv1.ContainerPort{
		{
			Name:          "fb-http-server",
			ContainerPort: 2020,
		},
	}
	// Note that the empty dir will be lost at restart. The idea is really to
	// only provide buffering to prevent OOM killing of the pod.
	storageEmptyDir := base.MkEmptyDirVolume(serviceName + "-fb-buf")
	storageVolumeMount := apiv1.VolumeMount{
		Name:      serviceName + "-fb-buf",
		MountPath: "/buffer-storage/",
	}
	container.Ports = ports
	container.VolumeMounts = append(volumeMounts, storageVolumeMount)
	return container, storageEmptyDir
}
