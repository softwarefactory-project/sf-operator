// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *SFKubeContext) CleanPVCs() {
	var pvcList apiv1.PersistentVolumeClaimList
	for range 60 {
		r.ListM(&pvcList)
		cleaned := true
		for _, pvc := range pvcList.Items {
			if pvc.Labels["app"] == "sf" && pvc.Labels["run"] != "gerrit" {
				ctrl.Log.Info("Deleting pvc", "name", pvc.Name, "status", pvc.Status.Phase)
				r.DeleteR(&pvc)
				cleaned = false
			}
		}
		if cleaned {
			break
		}
		time.Sleep(time.Second * 2)
	}
}

func (r *SFKubeContext) CleanSFInstance() {
	r.nukeZKClients()

	var cm apiv1.ConfigMap
	if r.GetM("sf-standalone-owner", &cm) {
		ctrl.Log.Info("Standalone mode detected. Deleting owner ConfigMap with foreground propagation.")
		r.DeleteR(&cm)
	} else {
		ctrl.Log.Info("No SoftwareFactory resource or standalone ConfigMap found.")
	}

	// Delete the resource manually to ensure they are gone before this function ends
	ctrl.Log.Info("Cleaning resources...")
	var svcList apiv1.ServiceList
	r.ListM(&svcList)
	for _, svc := range svcList.Items {
		if svc.Spec.Selector["app"] == "sf" && svc.Spec.Selector["run"] != "gerrit" {
			r.DeleteR(&svc)
		}
	}

	var depList appsv1.DeploymentList
	r.ListM(&depList)
	for _, dep := range depList.Items {
		if dep.Spec.Selector.MatchLabels["app"] == "sf" && dep.Spec.Selector.MatchLabels["run"] != "gerrit" {
			r.DeleteR(&dep)
		}
	}

	var stsList appsv1.StatefulSetList
	r.ListM(&stsList)
	for _, sts := range stsList.Items {
		if sts.Spec.Selector.MatchLabels["app"] == "sf" && sts.Spec.Selector.MatchLabels["run"] != "gerrit" {
			r.DeleteR(&sts)
		}
	}

	var podList apiv1.PodList
	for range 60 {
		r.ListM(&podList)
		cleaned := true
		for _, pod := range podList.Items {
			if pod.Labels["app"] == "sf" && pod.Labels["run"] != "gerrit" {
				ctrl.Log.Info("Deleting pod", "name", pod.Name, "status", pod.Status.Phase)
				r.DeleteR(&pod)
				cleaned = false
			}
		}
		if cleaned {
			break
		}
		time.Sleep(time.Second * 2)

	}
}
