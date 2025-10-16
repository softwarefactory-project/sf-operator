// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
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
			break
		}
		return string(corporateCM.ResourceVersion)
	} else {
		return "0"
	}
}

func (r *SFController) EnsureLogJuicer() bool {
	const (
		ident         = "logjuicer"
		port          = 3000
		pvcName       = "logjuicer-pvc"
		logJuicerData = "logjuicer-data"
	)

	// Ensure PVC exists
	storage := r.getStorageConfOrDefault(r.cr.Spec.Logjuicer)

	pvc := base.MkPVC(pvcName, r.ns, storage, apiv1.ReadWriteOnce)
	r.GetOrCreate(&pvc)

	// Create Service
	srv := base.MkService(ident, r.ns, ident, []int32{port}, ident, r.cr.Spec.ExtraLabels)
	r.GetOrCreate(&srv)

	// Create Deployment
	dep := base.MkDeployment(ident, r.ns, base.LogJuicerImage(), r.cr.Spec.ExtraLabels, r.isOpenShift)
	dep.Spec.Template.Spec.Containers[0].ImagePullPolicy = "Always"

	// Use PVC for logjuicer-data volume
	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkVolumePVC(logJuicerData, pvcName),
	}
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      logJuicerData,
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

	// Get all the configurations in one string, which are the environment varibles
	config := ""

	for _, v := range dep.Spec.Template.Spec.Containers[0].Env {
		config = config + v.Value
	}

	dep.Spec.Template.ObjectMeta.Annotations = map[string]string{
		"config-hash": utils.Checksum([]byte(config)),
		"serial":      "1",
		"certs":       r.AddCorporateCA(&dep.Spec.Template.Spec),
	}
	dep.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	// Reconcile deployment
	pvcReadiness := r.reconcileExpandPVC(pvcName, r.cr.Spec.Logjuicer)
	current, changed := r.ensureDeployment(dep)
	return !changed && r.IsDeploymentReady(current) && pvcReadiness
}
