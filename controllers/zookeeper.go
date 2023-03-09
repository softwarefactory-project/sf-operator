// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
)

//go:embed templates/zookeeper.yaml
var zk_objs string

func (r *SFController) DeployZookeeper() bool {
	r.CreateYAMLs(strings.ReplaceAll(zk_objs, "{{ NS }}", r.ns))
	cert := r.create_client_certificate(r.ns, "zookeeper-client", "ca-issuer", "zookeeper-client-tls", "zookeeper")
	r.GetOrCreate(&cert)
	var dep appsv1.StatefulSet
	r.GetM("zookeeper", &dep)
	return r.IsStatefulSetReady(&dep)
}
