// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed static/nodepool/nodepool.yaml
var nodepoolconf string

const NL_IDENT = "nodepool-launcher"
const NL_WEBAPP_PORT_NAME = "nlwebapp"
const NL_WEBAPP_PORT = 8006

func (r *SFController) DeployNodepool() bool {
	cert_client := r.create_client_certificate(r.ns, "zookeeper-client", "ca-issuer", "zookeeper-client-tls", "zookeeper")
	r.GetOrCreate(&cert_client)

	r.GetOrCreate(&apiv1.Secret{
		Data: map[string][]byte{
			"nodepool.yaml": []byte(nodepoolconf),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "nodepool-yaml", Namespace: r.ns},
	})

	annotations := map[string]string{
		"nodepool.yaml": checksum([]byte(nodepoolconf)),
	}

	nl := create_deployment(r.ns, "nodepool-launcher", "")
	volumes := []apiv1.Volume{
		create_volume_secret("nodepool-config", "nodepool-yaml"),
		create_volume_secret("zookeeper-client-tls"),
	}
	volume_mount := []apiv1.VolumeMount{
		{
			Name:      "zookeeper-client-tls",
			MountPath: "/tls/client",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-config",
			MountPath: "/etc/nodepool/nodepool.yaml",
			SubPath:   "nodepool.yaml",
		},
	}
	container := apiv1.Container{
		Name:            "launcher",
		Image:           "quay.io/software-factory/" + NL_IDENT + ":8.2.0-2",
		SecurityContext: create_security_context(false),
		VolumeMounts:    volume_mount,
	}
	nl.Spec.Template.Spec.Volumes = volumes
	nl.Spec.Template.Spec.Containers = []apiv1.Container{container}
	nl.Spec.Template.ObjectMeta.Annotations = annotations
	// FIXME: Add readiness and liveness probe when they are available.
	//nl.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/health/ready", 9090)
	//nl.Spec.Template.Spec.Containers[0].LivenessProbe = create_readiness_http_probe("/health/live", 9090)
	nl.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(NL_WEBAPP_PORT, NL_WEBAPP_PORT_NAME),
	}

	r.GetOrCreate(&nl)
	nl_dirty := false
	if !map_equals(&nl.Spec.Template.ObjectMeta.Annotations, &annotations) {
		nl.Spec.Template.ObjectMeta.Annotations = annotations
		nl_dirty = true
	}
	if nl_dirty {
		r.UpdateR(&nl)
	}

	return r.IsDeploymentReady(&nl)
}
