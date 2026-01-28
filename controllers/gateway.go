// Copyright (C) 2024 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the git-server configuration.

package controllers

import (
	_ "embed"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	apiv1 "k8s.io/api/core/v1"
)

//go:embed static/gateway/gateway.conf
var gatewayConfig string

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

	annotations := map[string]string{
		"config-hash": utils.Checksum([]byte(gatewayConfig)),
		"serial":      "1",
	}

	dep := base.MkDeployment(ident, r.Ns, base.HTTPDImage(), r.cr.Spec.ExtraLabels, r.IsOpenShift)

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
	dep.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	current, changed := r.ensureDeployment(dep, nil)
	return !changed && r.IsDeploymentReady(current)
}
