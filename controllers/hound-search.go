// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const houndSearchIdent = "hound-search"
const houndSearchImage = "quay.io/software-factory/hound:0.5.1-3"

func MkHoundSearchContainer(corporateCMExists bool, openshiftUser bool) apiv1.Container {
	container := base.MkContainer(houndSearchIdent, houndSearchImage, openshiftUser)
	container.Command = []string{"/sf-tooling/hound-search-init.sh"}
	container.Ports = []apiv1.ContainerPort{
		base.MkContainerPort(6080, houndSearchIdent),
	}
	container.ReadinessProbe = base.MkReadinessHTTPProbe("/healthz", 6080)
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
	if corporateCMExists {
		container.VolumeMounts = AppendCorporateCACertsVolumeMount(
			container.VolumeMounts, "hound-search-ca-certs")
		container.VolumeMounts = append(container.VolumeMounts,
			apiv1.VolumeMount{
				Name:      "hound-search-ca",
				MountPath: "/etc/pki/ca-trust/extracted",
			})
	}

	return container
}

func (r *SFController) TerminateHoundSearch() {
	r.DeleteR(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      houndSearchIdent,
			Namespace: r.ns,
		},
	})
	r.DeleteR(&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      houndSearchIdent,
			Namespace: r.ns,
		},
	})
	// todo: delete pvc
}

func (r *SFController) DeployHoundSearch() bool {
	svc := base.MkService(houndSearchIdent, r.ns, houndSearchIdent, []int32{6080}, houndSearchIdent, r.cr.Spec.ExtraLabels)
	r.EnsureService(&svc)

	// Check if Corporate Certificate exists
	corporateCM, corporateCMExists := r.CorporateCAConfigMapExists()

	pvc := base.MkPVC("hound-search-data", r.ns, r.getStorageConfOrDefault(r.cr.Spec.Codesearch.Storage), apiv1.ReadWriteOnce)
	container := MkHoundSearchContainer(corporateCMExists, r.isOpenShift)
	container.Env = []apiv1.EnvVar{
		base.MkEnvVar("CONFIG_REPO_BASE_URL", r.configBaseURL),
		base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigRepositoryLocation.Name),
	}
	sts := base.MkStatefulset(houndSearchIdent, r.ns, 1, houndSearchIdent, container, pvc, r.cr.Spec.ExtraLabels)
	sts.Spec.Template.Spec.Volumes = AppendToolingVolume(sts.Spec.Template.Spec.Volumes)
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, base.MkVolumeSecret("zuul-config"))

	if corporateCMExists {
		sts.Spec.Template.Spec.Volumes = append(
			sts.Spec.Template.Spec.Volumes,
			base.MkVolumeCM("hound-search-ca-certs", CorporateCACerts),
			base.MkEmptyDirVolume("hound-search-ca"))
	}

	annotations := map[string]string{
		"config-repo-info":           r.configBaseURL + r.cr.Spec.ConfigRepositoryLocation.Name,
		"corporate-ca-certs-version": getCMVersion(corporateCM, corporateCMExists),
		"image":                      houndSearchImage,
		"serial":                     "1",
		"config-scripts":             utils.Checksum([]byte(houndSearchRender + houndSearchInit + houndSearchConfig)),
	}
	limits := v1.LimitsSpec{
		CPU:    resource.MustParse("2000m"),
		Memory: resource.MustParse("2Gi"),
	}
	if r.cr.Spec.Codesearch.Limits != nil {
		limits.CPU = r.cr.Spec.Codesearch.Limits.CPU
		limits.Memory = r.cr.Spec.Codesearch.Limits.Memory
	}

	limitstr := base.UpdateContainerLimit(&limits, &sts.Spec.Template.Spec.Containers[0])
	annotations["limits"] = limitstr

	sts.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	sts.Spec.Template.ObjectMeta.Annotations = annotations
	current, stsUpdated := r.ensureStatefulset(sts)

	if stsUpdated {
		return false
	}

	if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
		utils.LogI("hound-search configuration changed, rollout pods ...")
		current.Spec = sts.DeepCopy().Spec
		r.UpdateR(current)
		return false
	}

	pvcReadiness := r.reconcileExpandPVC(
		houndSearchIdent+"-data-"+houndSearchIdent+"-0",
		r.cr.Spec.Codesearch.Storage)

	isReady := r.IsStatefulSetReady(current) && pvcReadiness
	conds.UpdateConditions(&r.cr.Status.Conditions, houndSearchIdent, isReady)

	return isReady
}
