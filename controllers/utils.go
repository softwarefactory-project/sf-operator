// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains common helper functions

package controllers

import (
	"github.com/google/uuid"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
)

func create_secret_env(env string, secret string) apiv1.EnvVar {
	return apiv1.EnvVar{
		Name: env,
		ValueFrom: &apiv1.EnvVarSource{
			SecretKeyRef: &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: secret,
				},
				Key: secret,
			},
		},
	}
}

func create_env(env string, value string) apiv1.EnvVar {
	return apiv1.EnvVar{
		Name:  env,
		Value: value,
	}
}

func create_container_port(port int, name string) apiv1.ContainerPort {
	return apiv1.ContainerPort{
		Name:          name,
		Protocol:      apiv1.ProtocolTCP,
		ContainerPort: int32(port),
	}
}

func create_volume_cm(name string, config_map_ref string) apiv1.Volume {
	return apiv1.Volume{
		Name: name,
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: config_map_ref,
				},
			},
		},
	}
}

// Create a default persistent volume claim.
func create_pvc(ns string, name string) apiv1.PersistentVolumeClaim {
	return apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{},
	}
}

// Create a default statefulset.
func create_statefulset(ns string, name string, image string) appsv1.StatefulSet {
	container := apiv1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
	}
	pvc := apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			// With kind, PVC should be automatically provisioned with https://github.com/rancher/local-path-provisioner
			// If the PVC is stuck in `Pending`, then a local hostPath must be created, e.g.:
			/*
			   apiVersion: v1
			   kind: PersistentVolume
			   metadata:
			     name: pv-kind4
			   spec:
			     storageClassName: standard
			     accessModes:
			       - ReadWriteOnce
			     capacity:
			       storage: 2Gi
			     hostPath:
			       path: /src/pvs4
			*/
			StorageClassName: strPtr("standard"),
			AccessModes:      []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					"storage": *resource.NewQuantity(1*1000*1000*1000, resource.DecimalSI),
				},
			},
		},
	}
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "sf",
					"run": name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "sf",
						"run": name,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						container,
					},
					AutomountServiceAccountToken: boolPtr(false),
				},
			},
			VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
				pvc,
			},
		},
	}
}

// Create a default deployment.
func create_deployment(ns string, name string, image string) appsv1.Deployment {
	container := apiv1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
	}
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "sf",
					"run": name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "sf",
						"run": name,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						container,
					},
					AutomountServiceAccountToken: boolPtr(false),
				},
			},
		},
	}
}

// create a default service.
func create_service(ns string, name string, port int32, port_name string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:     port_name,
					Protocol: apiv1.ProtocolTCP,
					Port:     port,
				},
			},
			Selector: map[string]string{
				"app": "sf",
				"run": name,
			},
		}}
}

func create_http_probe(path string, port int) *apiv1.Probe {
	return &apiv1.Probe{
		ProbeHandler: apiv1.ProbeHandler{
			HTTPGet: &apiv1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt(port),
			},
		},
		TimeoutSeconds:   3,
		PeriodSeconds:    5,
		FailureThreshold: 20,
	}
}

// Get a resources, returning if it was found
func (r *SFController) GetM(name string, obj client.Object) bool {
	err := r.Get(r.ctx,
		client.ObjectKey{
			Name:      name,
			Namespace: r.ns,
		}, obj)
	if errors.IsNotFound(err) {
		return false
	} else if err != nil {
		panic(err.Error())
	}
	return true
}

// Create resources with the controller reference
func (r *SFController) CreateR(obj client.Object) {
	controllerutil.SetControllerReference(r.cr, obj, r.Scheme)
	if err := r.Create(r.ctx, obj); err != nil {
		panic(err.Error())
	}
}

// generate a secret if needed using a uuid4 value.
func (r *SFController) EnsureSecret(name string) apiv1.Secret {
	var secret apiv1.Secret
	err := r.Get(r.ctx, client.ObjectKey{
		Name:      name,
		Namespace: r.ns,
	}, &secret)
	if errors.IsNotFound(err) {
		r.log.V(1).Info("Creating secret", "name", name)
		secret = apiv1.Secret{
			Data: map[string][]byte{
				// The data key is the same as the secret name.
				// This means that a Secret object presently only contains a single value.
				name: []byte(uuid.New().String()),
			},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: r.ns}}
		controllerutil.SetControllerReference(r.cr, &secret, r.Scheme)
		err = r.Create(r.ctx, &secret)
		if err != nil {
			panic(err.Error())
		}
	} else if err != nil && !errors.IsAlreadyExists(err) {
		panic(err.Error())
	}
	return secret
}

// ensure a config map exists.
func (r *SFController) EnsureConfigMap(base_name string, path string, data string) apiv1.ConfigMap {
	name := base_name + "-config-map"
	var cm apiv1.ConfigMap
	err := r.Get(r.ctx, client.ObjectKey{
		Name:      name,
		Namespace: r.ns,
	}, &cm)
	if errors.IsNotFound(err) {
		r.log.V(1).Info("Creating config", "name", name)
		cm = apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: r.ns},
			Data: map[string]string{
				path: data,
			},
		}
		controllerutil.SetControllerReference(r.cr, &cm, r.Scheme)
		err = r.Create(r.ctx, &cm)
		if err != nil {
			panic(err.Error())
		}
	} else if err != nil && !errors.IsAlreadyExists(err) {
		panic(err.Error())
	}
	return cm
}

func create_job(ns string, name string, container apiv1.Container) batchv1.Job {
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						container,
					},
					RestartPolicy: "Never",
				},
			},
		}}
}

func create_ingress_rule(host string, service string, port int) netv1.IngressRule {
	pt := netv1.PathTypePrefix
	return netv1.IngressRule{
		Host: host,
		IngressRuleValue: netv1.IngressRuleValue{
			HTTP: &netv1.HTTPIngressRuleValue{
				Paths: []netv1.HTTPIngressPath{
					{
						PathType: &pt,
						Path:     "/",
						Backend: netv1.IngressBackend{
							Service: &netv1.IngressServiceBackend{
								Name: service,
								Port: netv1.ServiceBackendPort{
									Number: int32(port),
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *SFController) SetupIngress(keycloakEnabled bool) {
	var ingress netv1.Ingress
	found := r.GetM(r.cr.Name, &ingress)
	ingress = netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.cr.Name,
			Namespace: r.ns,
		},
	}
	if r.cr.Spec.Etherpad {
		ingress.Spec.Rules = append(ingress.Spec.Rules, r.IngressEtherpad())
	}
	if keycloakEnabled {
		ingress.Spec.Rules = append(ingress.Spec.Rules, r.IngressKeycloak()...)
	}
	if !found {
		r.CreateR(&ingress)
	} else {
		if err := r.Update(r.ctx, &ingress); err != nil {
			panic(err.Error())
		}
	}
}

func int32Ptr(i int32) *int32 { return &i }
func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
