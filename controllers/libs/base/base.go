// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package base provides various utility functions regarding base k8s resources used by the sf-operator
package base

import (
	"fmt"

	apiroutev1 "github.com/openshift/api/route/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

// DefaultPodSecurityContext is the PodSecurityContext used by sf-operator Pods
var DefaultPodSecurityContext = apiv1.PodSecurityContext{
	RunAsNonRoot: pointer.Bool(true),
	SeccompProfile: &apiv1.SeccompProfile{
		Type: "RuntimeDefault",
	},
}

// MkSecurityContext produces a SecurityContext
func MkSecurityContext(privileged bool) *apiv1.SecurityContext {
	return &apiv1.SecurityContext{
		Privileged:               pointer.Bool(privileged),
		AllowPrivilegeEscalation: pointer.Bool(privileged),
		Capabilities: &apiv1.Capabilities{
			Drop: []apiv1.Capability{
				"ALL",
			},
		},
		SeccompProfile: &apiv1.SeccompProfile{
			Type: apiv1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// MkContainer produces a Container with the default settings
func MkContainer(name string, image string) apiv1.Container {
	return apiv1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: MkSecurityContext(false),
	}
}

// MkContainerPort produces a TCP ContainerPort
func MkContainerPort(port int, name string) apiv1.ContainerPort {
	return apiv1.ContainerPort{
		Name:          name,
		Protocol:      apiv1.ProtocolTCP,
		ContainerPort: int32(port),
	}
}

// MkVolumeSecret produces a Volume from a Secret source
// When the secretName var is not set then Secret name is the same as the Volume name
func MkVolumeSecret(name string, secretName ...string) apiv1.Volume {
	secName := name
	if secretName != nil {
		secName = secretName[0]
	}
	return apiv1.Volume{
		Name: name,
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: secName,
			},
		},
	}
}

// MkVolumeCM produce a Volume from a ConfigMap
func MkVolumeCM(volumeName string, configMapRef string) apiv1.Volume {
	return apiv1.Volume{
		Name: volumeName,
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: configMapRef,
				},
			},
		},
	}
}

// MkEmptyDirVolume produces a EmptyDir Volume
func MkEmptyDirVolume(name string) apiv1.Volume {
	return apiv1.Volume{
		Name: name,
		VolumeSource: apiv1.VolumeSource{
			EmptyDir: &apiv1.EmptyDirVolumeSource{},
		},
	}
}

// MkSecretEnvVar produces an EnvVar from a Secret's key.
// When the 'key' parameter is empty the key name is the Secret name
func MkSecretEnvVar(env string, secret string, key string) apiv1.EnvVar {
	if key == "" {
		key = secret
	}
	return apiv1.EnvVar{
		Name: env,
		ValueFrom: &apiv1.EnvVarSource{
			SecretKeyRef: &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: secret,
				},
				Key: key,
			},
		},
	}
}

// MkEnvVar is small helper to produce an EnvVar
func MkEnvVar(env string, value string) apiv1.EnvVar {
	return apiv1.EnvVar{
		Name:  env,
		Value: value,
	}
}

// MkSecretFromFunc produces a Secret where data is the result of getData
func MkSecretFromFunc(name string, namespace string, getData func() string) apiv1.Secret {
	return apiv1.Secret{
		Data:       map[string][]byte{name: []byte(getData())},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
}

// MkSSHKeySecret produces a Secret storing a SSH Key pair
func MkSSHKeySecret(name string, namespace string) apiv1.Secret {
	var secret apiv1.Secret
	sshkey := utils.MkSSHKey()
	secret = apiv1.Secret{
		Data: map[string][]byte{
			"priv": sshkey.Priv,
			"pub":  sshkey.Pub,
		},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	return secret
}

// StorageConfig is used to define PVC Storage
type StorageConfig struct {
	StorageClassName string
	Size             resource.Quantity
}

// MkPVC produces PerssistentVolumeClaim
func MkPVC(name string, ns string, storageParams StorageConfig, accessMode apiv1.PersistentVolumeAccessMode) apiv1.PersistentVolumeClaim {
	qty := storageParams.Size
	return apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			StorageClassName: &storageParams.StorageClassName,
			AccessModes:      []apiv1.PersistentVolumeAccessMode{accessMode},
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					"storage": qty,
				},
			},
		},
	}
}

// MkJob produces a Job
func MkJob(name string, ns string, container apiv1.Container) batchv1.Job {
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
					RestartPolicy:   "Never",
					SecurityContext: &DefaultPodSecurityContext,
				},
			},
		}}
}

// mkServicePorts produces a ServicePort array
func mkServicePorts(ports []int32, portName string) []apiv1.ServicePort {
	servicePorts := []apiv1.ServicePort{}
	for _, p := range ports {
		servicePorts = append(
			servicePorts,
			apiv1.ServicePort{
				Name:     fmt.Sprintf("%s-%d", portName, p),
				Protocol: apiv1.ProtocolTCP,
				Port:     p,
			})
	}
	return servicePorts
}

// MkService produces a Service
func MkService(name string, ns string, selector string, ports []int32, portName string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: mkServicePorts(ports, portName),
			Selector: map[string]string{
				"app": "sf",
				"run": selector,
			},
		}}
}

// MkServicePod produces a Service that target a single Pod by name
func MkServicePod(name string, ns string, podName string, ports []int32, portName string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: mkServicePorts(ports, portName),
			Selector: map[string]string{
				"statefulset.kubernetes.io/pod-name": podName,
			},
		}}

}

// MkHeadlessService produces a headless service.
func MkHeadlessService(name string, ns string, selector string, ports []int32, portName string) apiv1.Service {
	service := MkService(name, ns, selector, ports, portName)
	service.ObjectMeta.Name = name + "-headless"
	service.Spec.ClusterIP = "None"
	return service
}

// MkHTTPSRoute produces a Route on top of a Service
func MkHTTPSRoute(
	name string, ns string, host string, serviceName string, path string,
	port int, annotations map[string]string, fqdn string, customTLS *apiroutev1.TLSConfig) apiroutev1.Route {
	tls := apiroutev1.TLSConfig{
		InsecureEdgeTerminationPolicy: apiroutev1.InsecureEdgeTerminationPolicyRedirect,
		Termination:                   apiroutev1.TLSTerminationEdge,
	}
	if customTLS != nil {
		tls = *customTLS
	}
	return apiroutev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
		},
		Spec: apiroutev1.RouteSpec{
			TLS:  &tls,
			Host: host + "." + fqdn,
			To: apiroutev1.RouteTargetReference{
				Kind:   "Service",
				Name:   serviceName,
				Weight: pointer.Int32(100),
			},
			Port: &apiroutev1.RoutePort{
				TargetPort: intstr.FromInt(port),
			},
			Path:           path,
			WildcardPolicy: "None",
		},
	}
}

// MkStatefulset produces a StatefulSet.
func MkStatefulset(
	name string, ns string, replicas int32, serviceName string,
	container apiv1.Container, pvc apiv1.PersistentVolumeClaim) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    utils.Int32Ptr(replicas),
			ServiceName: serviceName,
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
					SecurityContext: &DefaultPodSecurityContext,
					Containers: []apiv1.Container{
						container,
					},
					AutomountServiceAccountToken: utils.BoolPtr(false),
				},
			},
			VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
				pvc,
			},
		},
	}
}

// MkDeployment produces a Deployment.
func MkDeployment(name string, ns string, image string) appsv1.Deployment {
	container := MkContainer(name, image)
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: utils.Int32Ptr(1),
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
					AutomountServiceAccountToken: utils.BoolPtr(false),
					SecurityContext:              &DefaultPodSecurityContext,
				},
			},
		},
	}
}

// IsStatefulSetRolloutDone returns True when the StatefulSet rollout is over
func IsStatefulSetRolloutDone(obj *appsv1.StatefulSet) bool {
	return obj.Status.ObservedGeneration >= obj.Generation &&
		obj.Status.Replicas == obj.Status.ReadyReplicas &&
		obj.Status.Replicas == obj.Status.CurrentReplicas
}

// IsDeploymentRolloutDone returns True when the Deployment rollout is over
func IsDeploymentRolloutDone(obj *appsv1.Deployment) bool {
	return obj.Status.ObservedGeneration >= obj.Generation &&
		obj.Status.Replicas == obj.Status.ReadyReplicas &&
		obj.Status.Replicas == obj.Status.AvailableReplicas
}

func IsDeploymentReady(dep *appsv1.Deployment) bool {
	return dep.Status.ReadyReplicas > 0 && IsDeploymentRolloutDone(dep)
}
