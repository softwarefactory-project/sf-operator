// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
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
		"zuul-conf": checksum,
		"serial":    "1",
	}

	dep := base.MkDeployment(ident, r.ns, "ghcr.io/softwarefactory-project/zuul-weeder:latest", r.cr.Spec.ExtraLabels, r.isOpenShift)
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
	current := appsv1.Deployment{}
	if r.GetM(ident, &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			utils.LogI("zuul configuration changed, rollout zuul-weeder pods ...")
			current.Spec = dep.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := dep
		r.CreateR(&current)
	}

	return r.IsDeploymentReady(&current)
}
