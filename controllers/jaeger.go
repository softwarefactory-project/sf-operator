// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the jaeger configuration.
package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const JAEGER_PORT = 16686
const JAEGER_PORT_NAME = "jaeger"

func (r *SFController) DeployJaeger(enabled bool) bool {
	if enabled {
		dep := create_deployment(r.ns, "jaeger", "quay.io/jaegertracing/all-in-one:latest")
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_env("COLLECTOR_OTLP_ENABLED", "true"),
			create_env("SPAN_STORAGE_TYPE", "badger"),
			create_env("BADGER_EPHEMERAL", "false"),
			create_env("BADGER_DIRECTORY_VALUE", "/badger/data"),
			create_env("BADGER_DIRECTORY_KEY", "/badger/key"),
		}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{Name: "badger", MountPath: "/badger"},
		}
		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{create_empty_dir("badger")}
		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(JAEGER_PORT, JAEGER_PORT_NAME),
			create_container_port(4317, "oltp-grpc"),
			create_container_port(4318, "oltp-http"),
		}
		r.GetOrCreate(&dep)
		srv := create_service(r.ns, "jaeger", "jaeger", JAEGER_PORT, JAEGER_PORT_NAME)
		r.GetOrCreate(&srv)
		grpc := create_service(r.ns, "oltp-grpc", "oltp-grpc", 4317, "oltp-grpc")
		r.GetOrCreate(&grpc)
		http := create_service(r.ns, "oltp-http", "oltp-http", 4318, "oltp-http")
		r.GetOrCreate(&http)
		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment("jaeger")
		r.DeleteService("jaeger")
		r.DeleteService("oltp-grpc")
		r.DeleteService("oltp-http")
		return true
	}
}

func (r *SFController) IngressJaeger() netv1.IngressRule {
	return create_ingress_rule("jaeger."+r.cr.Spec.FQDN, JAEGER_PORT_NAME, JAEGER_PORT)
}
