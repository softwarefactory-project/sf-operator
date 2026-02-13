// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

// This package contains wrapper to low-level client Get/Create/Update/Delete
// to perform the following:
//
// - Log error
// - Scope request to the client namespace

package client

import (
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Get gets a resource, returning true if it was found
func (r *KubeClient) Get(name string, obj client.Object) (bool, error) {
	err := r.Client.Get(r.Ctx,
		client.ObjectKey{
			Name:      name,
			Namespace: r.Ns,
		},
		obj)
	if errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		ctrl.Log.Error(err, "Failed to get resource", "name", obj.GetName(), "kind", obj.GetObjectKind().GroupVersionKind().Kind)
	}
	return true, err
}

func (r *KubeClient) GetOrDie(name string, obj client.Object) bool {
	exist, err := r.Get(name, obj)
	if err != nil {
		panic(err)
	}
	return exist
}

func (r *KubeClient) List(list client.ObjectList) error {
	opts := []client.ListOption{client.InNamespace(r.Ns)}
	err := r.Client.List(r.Ctx, list, opts...)
	if err != nil {
		ctrl.Log.Error(err, "Failed to list resource", "kind", list.GetObjectKind().GroupVersionKind().Kind)
	}
	return err
}

func (r *KubeClient) ListOrDie(list client.ObjectList) {
	if err := r.List(list); err != nil {
		panic(err)
	}
}
