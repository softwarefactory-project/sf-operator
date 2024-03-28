// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the git-server configuration.

package controllers

import (
	_ "embed"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

//go:embed static/gateway/gateway.conf
var gatewayConfig string

func (r *SFController) DeployHTTPDGateway() bool {

	const (
		ident = "gateway"
		port  = 8080
	)

	srv := base.MkService(ident, r.ns, ident, []int32{port}, ident)
	r.GetOrCreate(&srv)

	r.EnsureConfigMap(ident, map[string]string{
		"gateway.conf": gatewayConfig,
	})

	annotations := map[string]string{
		"image":      base.HTTPDImage(),
		"httpd-conf": utils.Checksum([]byte(gatewayConfig)),
		"serial":     "1",
	}

	dep := base.MkDeployment(ident, r.ns, base.HTTPDImage())
	dep.Spec.Template.ObjectMeta.Annotations = annotations
	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkVolumeCM(ident, ident+"-config-map"),
	}
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      ident,
			MountPath: "/etc/httpd/conf.d/gateway.conf",
			ReadOnly:  true,
			SubPath:   "gateway.conf",
		},
	}

	current := appsv1.Deployment{}
	if r.GetM(ident, &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("gateway configuration changed, rollout gateway pods ...")
			current.Spec = dep.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := dep
		r.CreateR(&current)
	}

	isDeploymentReady := r.IsDeploymentReady(&current)

	routeReady := r.ensureHTTPSRoute(
		ident, r.cr.Spec.FQDN,
		ident, "/", port, map[string]string{}, r.cr.Spec.LetsEncrypt)

	isReady := isDeploymentReady && routeReady

	return isReady
}
