// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the git-server configuration.

package controllers

import (
	_ "embed"
	"errors"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	apiv1 "k8s.io/api/core/v1"
)

//go:embed static/gateway/gateway.conf
var gatewayConfig string

func logCMError(cmName string) {
	cmError := errors.New("ConfigMap missing: " + cmName)
	logging.LogE(cmError, "Please create the configmap or remove it from your Software Factory manifest")
}

func (r *SFController) DeployHTTPDGateway() bool {

	const (
		ident = "gateway"
		port  = 8080
	)

	srv := base.MkService(ident, r.Ns, ident, []int32{port}, ident, r.cr.Spec.ExtraLabels)
	r.GetOrCreate(&srv)

	r.EnsureConfigMap(ident, map[string]string{
		"gateway.conf": gatewayConfig,
	})

	volumes := []apiv1.Volume{
		base.MkVolumeCM(ident, ident+"-config-map"),
	}
	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      ident,
			MountPath: "/etc/httpd/conf.d/99-gateway.conf",
			ReadOnly:  true,
			SubPath:   "gateway.conf",
		},
	}

	configHash := gatewayConfig

	gatewaySpec := r.cr.Spec.Gateway
	if gatewaySpec != nil {
		extraConfigCMName := gatewaySpec.ExtraConfigurationConfigMap
		var extraConfigCM apiv1.ConfigMap
		if !r.GetOrDie(extraConfigCMName, &extraConfigCM) {
			logCMError(extraConfigCMName)
			return false
		}
		for f, contents := range extraConfigCM.Data {
			configHash += "-" + f + "-" + contents
			// volumes can't be mounted within a volume so we need to use subpaths
			var extraConfigVolumeMount = apiv1.VolumeMount{
				Name:      extraConfigCMName,
				MountPath: "/etc/httpd/conf.d/" + f,
				ReadOnly:  true,
				SubPath:   f,
			}
			volumeMounts = append(volumeMounts, extraConfigVolumeMount)
		}

		extraConfigVolume := base.MkVolumeCM(extraConfigCMName, extraConfigCMName)
		volumes = append(volumes, extraConfigVolume)

		extraStaticFilesCMName := gatewaySpec.ExtraStaticFilesConfigMap
		if extraStaticFilesCMName != nil {
			var extraStaticFilesCM apiv1.ConfigMap
			if !r.GetOrDie(*extraStaticFilesCMName, &extraStaticFilesCM) {
				logCMError(*extraStaticFilesCMName)
				return false
			}
			for f, contents := range extraStaticFilesCM.Data {
				configHash += "-" + f + "-" + contents
			}

			extraStaticFilesVolumeMount := apiv1.VolumeMount{
				Name:      *extraStaticFilesCMName,
				MountPath: "/var/www/html/",
				ReadOnly:  true,
			}
			extraStaticFilesVolume := base.MkVolumeCM(*extraStaticFilesCMName, *extraStaticFilesCMName)

			volumes = append(volumes, extraStaticFilesVolume)
			volumeMounts = append(volumeMounts, extraStaticFilesVolumeMount)
		}

	}

	dep := base.MkDeployment(ident, r.Ns, base.HTTPDImage(), r.cr.Spec.ExtraLabels, r.IsOpenShift)

	dep.Spec.Template.Spec.Volumes = volumes
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	dep.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	// Annotation bump to 2 to force a renaming of the default config file
	annotations := map[string]string{
		"config-hash": utils.Checksum([]byte(configHash)),
		"serial":      "2",
	}
	dep.Spec.Template.ObjectMeta.Annotations = annotations

	current, changed := r.ensureDeployment(dep, nil)
	return !changed && r.IsDeploymentReady(current)
}
