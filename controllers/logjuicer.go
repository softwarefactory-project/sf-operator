// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

func (r *SFController) EnsureLogJuicer() bool {
	const (
		ident = "logjuicer"
		port  = 3000
	)
	current := appsv1.Deployment{}
	if r.GetM(ident, &current) {
		return r.IsDeploymentReady(&current)
	} else {
		srv := base.MkService(ident, r.ns, ident, []int32{port}, ident, r.cr.Spec.ExtraLabels)
		r.GetOrCreate(&srv)

		dep := base.MkDeployment(ident, r.ns, "ghcr.io/logjuicer/logjuicer:latest", r.cr.Spec.ExtraLabels)
		dep.Spec.Template.Spec.Containers[0].ImagePullPolicy = "Always"
		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			// TODO: make this persistent
			base.MkEmptyDirVolume("logjuicer-data"),
		}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      "logjuicer-data",
				MountPath: "/data",
			},
		}
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			base.MkEnvVar("LOGJUICER_BASE_URL", "/logjuicer/"),
		}
		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			base.MkContainerPort(port, ident),
		}
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/ready", port)

		r.CreateR(&dep)
		return false
	}
}
