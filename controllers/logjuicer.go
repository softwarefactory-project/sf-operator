// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

func (r *SFController) AddCorporateCA(spec *apiv1.PodSpec) string {
	// Inject into the spec the necessary option to setup the corporate-ca-certs, returns the current version
	corporateCM, corporateCMExists := r.CorporateCAConfigMapExists()
	if corporateCMExists {
		for fileName := range corporateCM.Data {
			spec.Volumes = append(spec.Volumes, base.MkVolumeCM("certs", CorporateCACerts))
			spec.Containers[0].VolumeMounts = append(spec.Containers[0].VolumeMounts, apiv1.VolumeMount{
				Name:      "certs",
				MountPath: "/certs",
			})
			spec.Containers[0].Env = append(spec.Containers[0].Env, base.MkEnvVar("LOGJUICER_CA_EXTRA", "/certs/"+fileName))
			// TODO: remove the next line after merging https://github.com/logjuicer/logjuicer/pull/144
			spec.Containers[0].Env = append(spec.Containers[0].Env, base.MkEnvVar("LOGJUICER_CA_BUNDLE", "/certs/"+fileName))
			break
		}
		return string(corporateCM.ResourceVersion)
	} else {
		return "0"
	}
}

func (r *SFController) EnsureLogJuicer() bool {
	const (
		ident = "logjuicer"
		port  = 3000
	)

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

	dep.Spec.Template.ObjectMeta.Annotations = map[string]string{
		"certs": r.AddCorporateCA(&dep.Spec.Template.Spec),
	}
	dep.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	current := appsv1.Deployment{}
	if r.GetM(ident, &current) {
		if utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &dep.Spec.Template.ObjectMeta.Annotations) {
			return r.IsDeploymentReady(&current)
		}
		current.Spec = dep.Spec
		r.UpdateR(&current)
	} else {
		r.CreateR(&dep)
	}
	return false
}
