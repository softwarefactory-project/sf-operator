// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"fmt"
	"os"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kclient "github.com/softwarefactory-project/sf-operator/controllers/libs/client"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
)

type SFKubeContext struct {
	kclient.KubeClient
	hasProcMount bool
}

func MkSFKubeContext(kubeconfig string, namespace string, kubecontext string, dryRun bool) (SFKubeContext, error) {
	client, err := kclient.MkKubeClient(kubeconfig, kubecontext, namespace, dryRun)
	if err != nil {
		return SFKubeContext{}, err
	}
	return SFKubeContext{
		KubeClient:   client,
		hasProcMount: os.Getenv("HAS_PROC_MOUNT") == "true",
	}, nil
}

func (r *SFKubeContext) ScaleDownSTSAndWait(name string) error {
	var sts appsv1.StatefulSet
	key := client.ObjectKey{Namespace: r.Ns, Name: name}

	// 1. Get the StatefulSet
	if err := r.Client.Get(r.Ctx, key, &sts); err != nil {
		if apierrors.IsNotFound(err) {
			ctrl.Log.Info("StatefulSet not found, nothing to do.", "name", name)
			return nil // The goal is achieved if it doesn't exist.
		}
		ctrl.Log.Error(err, "Failed to get StatefulSet", "name", name)
		return err
	}

	// 2. Scale replicas to 0
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 0 {
		ctrl.Log.Info("Scaling down StatefulSet replicas to 0", "name", name)
		sts.Spec.Replicas = ptr.To[int32](0)
		if err := r.Client.Update(r.Ctx, &sts); err != nil {
			ctrl.Log.Error(err, "Failed to update StatefulSet replicas", "name", name)
			return err
		}
	} else {
		ctrl.Log.Info("StatefulSet already has 0 replicas", "name", name)
	}

	// 3. Wait for all pods managed by the StatefulSet to terminate
	ctrl.Log.Info("Waiting for pods to terminate...", "statefulset", name)
	selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
	if err != nil {
		ctrl.Log.Error(err, "Failed to build label selector from StatefulSet", "name", name)
		return err
	}

	for range 10 {
		var podList apiv1.PodList
		listOpts := []client.ListOption{
			client.InNamespace(r.Ns),
			client.MatchingLabelsSelector{Selector: selector},
		}
		if err := r.Client.List(r.Ctx, &podList, listOpts...); err != nil {
			ctrl.Log.Error(err, "Unable to list statefulset pods")
			return err
		}
		if len(podList.Items) == 0 {
			break // Done: no pods found
		}
		ctrl.Log.Info("Waiting, pods still present", "name", name, "count", len(podList.Items))
		time.Sleep(5 * time.Second)
		continue // Not done yet
	}

	ctrl.Log.Info("All pods for StatefulSet have terminated.", "name", name)
	return nil
}

func (r *SFKubeContext) DeleteOrDie(obj client.Object, opts ...client.DeleteOption) bool {
	err := r.Client.Delete(r.Ctx, obj, opts...)
	if apierrors.IsNotFound(err) {
		return false
	} else if err != nil {
		msg := fmt.Sprintf("Error while deleting %s \"%s\"", reflect.TypeOf(obj).Name(), obj.GetName())
		logging.LogE(err, msg)
		os.Exit(1)
	}
	return true
}

func (r *SFKubeContext) UpdateROrDie(obj client.Object) {
	var msg = fmt.Sprintf("Updating %s \"%s\" in %s", reflect.TypeOf(obj).Name(), obj.GetName(), r.Ns)
	logging.LogI(msg)
	if err := r.Client.Update(r.Ctx, obj); err != nil {
		msg = fmt.Sprintf("Error while updating %s \"%s\"", reflect.TypeOf(obj).Name(), obj.GetName())
		logging.LogE(err, msg)
		os.Exit(1)
	}
	msg = fmt.Sprintf("%s \"%s\" updated", reflect.TypeOf(obj).Name(), obj.GetName())
	logging.LogI(msg)
}

func (r *SFKubeContext) CreateROrDie(obj client.Object) {
	var msg = fmt.Sprintf("Creating %s \"%s\" in %s", reflect.TypeOf(obj).Name(), obj.GetName(), r.Ns)
	logging.LogI(msg)
	obj.SetNamespace(r.Ns)
	if err := r.Client.Create(r.Ctx, obj); err != nil {
		msg = fmt.Sprintf("Error while creating %s \"%s\"", reflect.TypeOf(obj).Name(), obj.GetName())
		logging.LogE(err, msg)
		os.Exit(1)
	}
	msg = fmt.Sprintf("%s \"%s\" created", reflect.TypeOf(obj).Name(), obj.GetName())
	logging.LogI(msg)
}

func (r *SFKubeContext) DeleteAllOfOrDie(obj client.Object, opts ...client.DeleteAllOfOption) {
	if err := r.Client.DeleteAllOf(r.Ctx, obj, opts...); err != nil {
		var msg = "Error while deleting"
		logging.LogE(err, msg)
		os.Exit(1)
	}
}

func (r *SFKubeContext) EnsureNamespaceOrDie(name string) {
	var ns apiv1.Namespace
	if err := r.Client.Get(r.Ctx, client.ObjectKey{Name: name}, &ns); apierrors.IsNotFound(err) {
		ns.Name = name
		r.CreateROrDie(&ns)
	} else if err != nil {
		logging.LogE(err, "Error checking namespace "+name)
		os.Exit(1)
	}
}
func (r *SFKubeContext) EnsureServiceAccountOrDie(name string) {
	var sa apiv1.ServiceAccount
	found := r.GetOrDie(name, &sa)
	if !found {
		sa.Name = name
		r.CreateROrDie(&sa)
	}
}
