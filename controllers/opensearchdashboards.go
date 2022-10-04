// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the opensearch dashboards configuration.
package controllers

import (
	_ "embed"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

//go:embed static/opensearch-dashboards/opensearch_dashboards.yml
var os_dashboards_objs string

const DASHBOARDS_PORT = 5601
const DASHBOARDS_PORT_NAME = "os"

func (r *SFController) DeployOpensearchDashboards(enabled bool) bool {
	if enabled {
		r.log.V(1).Info("Opensearch Dashboards deploy not found")
		// create cert
		server_cert := r.create_client_certificate(r.ns, "opensearch-dashboards", "ca-issuer", "opensearch-dashboards-tls", "opensearchdashboards")
		r.GetOrCreate(&server_cert)

		// replace some string in the config file
		cm_data := make(map[string]string)
		os_dashboards_objs = strings.ReplaceAll(os_dashboards_objs, "{{ NS }}", r.ns)
		cm_data["opensearch_dashboards.yml"] = os_dashboards_objs
		r.EnsureConfigMap("opensearch-dashboards", cm_data)

		dep := create_statefulset(r.ns, "opensearch-dashboards", "quay.io/software-factory/opensearch-dashboards:2.2.0-1")
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"/bin/bash", "-x", "/usr/share/opensearch-dashboards/opensearch-dashboards-docker-entrypoint.sh"}

		dep.Spec.Template.Spec.Hostname = "opensearch-dashboards"
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(DASHBOARDS_PORT)

		user := int64(1000)
		securitycontext := &apiv1.PodSecurityContext{RunAsUser: &user, RunAsGroup: &user, FSGroup: &user}
		dep.Spec.Template.Spec.SecurityContext = securitycontext

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(DASHBOARDS_PORT, DASHBOARDS_PORT_NAME),
		}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      "opensearch-dashboards-config",
				MountPath: "/usr/share/opensearch-dashboards/config/opensearch_dashboards.yml",
				SubPath:   "opensearch_dashboards.yml",
				ReadOnly:  true,
			},
			{
				Name:      "opensearch-dashboards-certs",
				MountPath: "/usr/share/opensearch-dashboards/config/certs",
				ReadOnly:  true,
			},
		}
		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm("opensearch-dashboards-config", "opensearch-dashboards-config-map"),
			{
				Name: "opensearch-dashboards-certs",
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName: "opensearch-dashboards-tls",
					},
				},
			},
		}

		// Expose env vars
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			// env vars available in opensearch dashboards container by executing:
			// bash -x opensearch-dashboards-docker-entrypoint.sh
			create_env("OPENSEARCH_USERNAME", "kibanaserver"),
			create_secret_env("OPENSEARCH_PASSWORD", "opensearch-kibanaserver-password", "opensearch-kibanaserver-password"),
		}

		r.GetOrCreate(&dep)
		srv := create_service(r.ns, "opensearch-dashboards", "opensearch-dashboards", DASHBOARDS_PORT, DASHBOARDS_PORT_NAME)
		r.GetOrCreate(&srv)
		return r.IsStatefulSetReady(&dep)
	} else {
		r.DeleteStatefulSet("opensearch-dashboards")
		r.DeleteService("opensearch-dashboards")
		return true
	}
}
func (r *SFController) IngressOpensearchDashboards() netv1.IngressRule {
	return create_ingress_rule("opensearch-dashboards."+r.cr.Spec.FQDN, "opensearch-dashboards", DASHBOARDS_PORT)
}
