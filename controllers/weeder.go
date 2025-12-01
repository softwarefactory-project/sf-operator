// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	apiv1 "k8s.io/api/core/v1"
)

func (r *SFController) EnsureZuulWeeder(checksum string) bool {
	const (
		ident = "zuul-weeder"
		port  = 9001
	)

	srv := base.MkService(ident, r.ns, ident, []int32{port}, ident, r.cr.Spec.ExtraLabels)
	r.GetOrCreate(&srv)

	annotations := map[string]string{
		"zuul-connections": checksum,
		"serial":           "2",
	}

	dep := base.MkDeployment(ident, r.ns, base.ZuulWeederImage(), r.cr.Spec.ExtraLabels, r.isOpenShift)
	dep.Spec.Template.Spec.Containers[0].ImagePullPolicy = "Always"
	dep.Spec.Template.ObjectMeta.Annotations = annotations
	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkEmptyDirVolume("weeder-tmp"),
		base.MkVolumeSecret("zuul-config"),
		base.MkVolumeSecret("zookeeper-client-tls"),
	}
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      "weeder-tmp",
			MountPath: "/var/tmp/weeder",
		},
		{
			Name:      "zuul-config",
			MountPath: "/etc/zuul",
			ReadOnly:  true,
		},
		{
			Name:      "zookeeper-client-tls",
			MountPath: "/tls/client",
			ReadOnly:  true,
		},
	}
	dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		base.MkEnvVar("WEEDER_ROOT_URL", "/weeder"),
	}
	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(port, ident),
	}
	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/health", port)
	dep.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)
	current, changed := r.ensureDeployment(dep, nil)
	return !changed && r.IsDeploymentReady(current)
}
