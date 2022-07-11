// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed templates/nodepool.yaml
var nodepool_objs string

func (r *SFController) EnsureNodepoolSecrets() {
	r.Apply(&apiv1.Secret{
		Data: map[string][]byte{
			"nodepool.yaml": []byte(`
labels: []
providers: []
`),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "nodepool-yaml", Namespace: r.ns},
	})

}

func (r *SFController) DeployNodepool(enabled bool) bool {
	if enabled {
		r.EnsureNodepoolSecrets()
		r.CreateYAMLs(nodepool_objs)
		return r.IsDeploymentReady("nodepool-launcher")
	} else {
		r.DeleteDeployment("nodepool-launcher")
		return true
	}
}
