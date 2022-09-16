// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed templates/nodepool.yaml
var nodepool_objs string

func (r *SFController) EnsureNodepoolSecrets() {
	r.GetOrCreate(&apiv1.Secret{
		Data: map[string][]byte{
			"nodepool.yaml": []byte(`
labels: []
providers: []
`),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "nodepool-yaml", Namespace: r.ns},
	})

}

func (r *SFController) DeployNodepool() bool {
	// Nodepool is enable if Zuul is enabled
	if r.cr.Spec.Zuul.Enabled {
		r.EnsureNodepoolSecrets()
		r.CreateYAMLs(nodepool_objs)
		var dep appsv1.Deployment
		r.GetM("nodepool-launcher", &dep)
		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment("nodepool-launcher")
		return true
	}
}
