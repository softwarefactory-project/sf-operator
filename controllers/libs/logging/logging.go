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

type PythonTemplateLoggingParams struct {
	LogLevel    string
	ForwardLogs bool
	BaseURL     string
}

func CreateForwarderEnvVars(name string, extraLabels []FluentBitLabel) []apiv1.EnvVar {
	forwarderEnvVars := []apiv1.EnvVar{
		base.MkEnvVarFromFieldRef("K8S_NODENAME", "spec.nodeName"),
		base.MkEnvVarFromFieldRef("K8S_PODNAME", "metadata.name"),
		base.MkEnvVarFromFieldRef("K8S_NAMESPACE", "metadata.namespace"),
		base.MkEnvVarFromFieldRef("K8S_PODIP", "status.podIP"),
		base.MkEnvVar("K8S_LABELS_RUN", name),
	}
	for i := range extraLabels {
		var v = base.MkEnvVar("K8S_"+extraLabels[i].Key, extraLabels[i].Value)
		forwarderEnvVars = append(forwarderEnvVars, v)
	}
	return forwarderEnvVars
}

func SetupLogForwarding(serviceName string, forwarderSpec *v1.FluentBitForwarderSpec, extraLabels []FluentBitLabel, annotations map[string]string) []apiv1.EnvVar {
	if forwarderSpec != nil {
		annotations["log-forwarding"] = forwarderSpec.HTTPInputHost + ":" + strconv.Itoa(int(forwarderSpec.HTTPInputPort))
		return CreateForwarderEnvVars(serviceName, extraLabels)
	} else {
		annotations["log-forwarding"] = "disabled"
		return []apiv1.EnvVar{}
	}
}
