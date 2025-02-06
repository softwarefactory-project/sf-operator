// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	apiv1 "k8s.io/api/core/v1"
)

func MkHoundSearchContainer() apiv1.Container {
	container := base.MkContainer("hound-search", "quay.io/software-factory/hound:0.5.1-3")
	container.Command = []string{"/sf-tooling/hound-search-init.sh"}
	container.Ports = []apiv1.ContainerPort{
		base.MkContainerPort(6080, "hound-search"),
	}
	container.ReadinessProbe = base.MkReadinessHTTPProbe("/", 6080)
	container.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      "hound-search-data",
			MountPath: "/var/lib/hound",
		},
		{
			Name:      "zuul-config",
			MountPath: "/etc/zuul",
			ReadOnly:  true,
		},
		{
			Name:      "tooling-vol",
			MountPath: "/sf-tooling",
			ReadOnly:  true,
		},
	}
	return container
}

func (r *SFController) DeployHoundSearch() bool {
	svc := base.MkService("hound-search", r.ns, "hound-search", []int32{6080}, "hound-search", r.cr.Spec.ExtraLabels)
	r.EnsureService(&svc)
	pvc := base.MkPVC("hound-search-data", r.ns, BaseGetStorageConfOrDefault(v1.StorageSpec{}, r.cr.Spec.StorageDefault), apiv1.ReadWriteOnce)
	container := MkHoundSearchContainer()
	container.Env = []apiv1.EnvVar{
		base.MkEnvVar("CONFIG_REPO_BASE_URL", r.cr.Spec.ConfigRepositoryLocation.BaseURL),
		base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigRepositoryLocation.Name),
	}
	sts := base.MkStatefulset("hound-search", r.ns, 1, "hound-search", container, pvc, r.cr.Spec.ExtraLabels)
	sts.Spec.Template.Spec.Volumes = AppendToolingVolume(sts.Spec.Template.Spec.Volumes)
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, base.MkVolumeSecret("zuul-config"))

	annotations := map[string]string{
		"config-repo-info": r.cr.Spec.ConfigRepositoryLocation.BaseURL + r.cr.Spec.ConfigRepositoryLocation.Name,
	}
	sts.Spec.Template.ObjectMeta.Annotations = annotations
	current, stsUpdated := r.ensureStatefulset(sts)
	if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
		utils.LogI("hound configuration changed, rollout pods ...")
		current.Spec = sts.DeepCopy().Spec
		r.UpdateR(current)
		return false
	}
	return !stsUpdated
}
