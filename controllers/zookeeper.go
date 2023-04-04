// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
)

var zkTemplate = "controllers/templates/zookeeper.yaml"

type ZookeeperConfig struct {
	Namespace    string
	StorageClass string
	StorageSize  string
}

func getZKStorageSize(spec sfv1.SoftwareFactorySpec) string {
	if spec.Zookeeper.StorageSize != "" {
		return spec.Zookeeper.StorageSize
	} else {
		return "1Gi"
	}
}

func (r *SFController) DeployZookeeper() bool {
	zookeeperConfig := ZookeeperConfig{
		Namespace:    r.ns,
		StorageClass: get_storage_classname(r.cr.Spec),
		StorageSize:  getZKStorageSize(r.cr.Spec),
	}

	zookeeper_yaml, err := parse_template(zkTemplate, zookeeperConfig)
	if err != nil {
		r.log.V(1).Error(err, "Template parsing failed")
	}
	r.CreateYAMLs(zookeeper_yaml)
	cert := r.create_client_certificate(r.ns, "zookeeper-client", "ca-issuer", "zookeeper-client-tls", "zookeeper")
	r.GetOrCreate(&cert)
	var dep appsv1.StatefulSet
	r.GetM("zookeeper", &dep)
	return r.IsStatefulSetReady(&dep)
}
