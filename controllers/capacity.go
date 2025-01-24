// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	apiv1 "k8s.io/api/core/v1"
)

func MkZuulCapacityContainer() apiv1.Container {
	container := base.MkContainer("zuul-capacity", "ghcr.io/softwarefactory-project/zuul-capacity:latest")
	container.Args = []string{"--port", "9100"}
	container.Env = []apiv1.EnvVar{
		base.MkEnvVar("OS_CLIENT_CONFIG_FILE", "/.openstack/clouds.yaml"),
	}
	container.Ports = []apiv1.ContainerPort{
		base.MkContainerPort(9100, "zuul-capacity"),
	}
	container.ReadinessProbe = base.MkReadinessHTTPProbe("/", 9100)
	container.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      "nodepool-config",
			MountPath: "/etc/nodepool",
			ReadOnly:  true,
		},
		{
			Name:      "zookeeper-client-tls",
			MountPath: "/tls/client",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-providers-secrets",
			SubPath:   "clouds.yaml",
			MountPath: "/.openstack/clouds.yaml",
			ReadOnly:  true,
		},
	}
	return container
}