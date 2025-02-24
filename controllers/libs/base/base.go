// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package base provides various utility functions regarding base k8s resources used by the sf-operator
package base

import (
	"fmt"
	"maps"

	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// DefaultPodSecurityContext is the PodSecurityContext used by sf-operator Pods
var DefaultPodSecurityContext = apiv1.PodSecurityContext{
	RunAsNonRoot: ptr.To(true),
	SeccompProfile: &apiv1.SeccompProfile{
		Type: "RuntimeDefault",
	},
}

// MkSecurityContext produces a SecurityContext
func MkSecurityContext(privileged bool) *apiv1.SecurityContext {
	return &apiv1.SecurityContext{
		Privileged:               ptr.To(privileged),
		AllowPrivilegeEscalation: ptr.To(privileged),
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

// SetContainerLimits sets the Resource limit according to
// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
func SetContainerLimits(container *apiv1.Container, memRequest resource.Quantity, memLimit resource.Quantity, cpuRequest resource.Quantity, cpuLimit resource.Quantity) {
	var (
		defaultResources = apiv1.ResourceRequirements{
			Requests: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: memRequest,
				apiv1.ResourceCPU:    cpuRequest,
			},
			Limits: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: memLimit,
				apiv1.ResourceCPU:    cpuLimit,
			},
		}
	)
	container.Resources = defaultResources
}

// MkContainer produces a Container with the default settings
func MkContainer(name string, image string) apiv1.Container {
	var container = apiv1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: MkSecurityContext(false),
	}
	setContainerLimitsDefaultProfile(&container)
	return container
}

func setContainerLimitsDefaultProfile(container *apiv1.Container) {
	SetContainerLimits(
		container,
		resource.MustParse("128Mi"),
		resource.MustParse("256Mi"),
		resource.MustParse("100m"),
		resource.MustParse("500m"))
}

func SetContainerLimitsLowProfile(container *apiv1.Container) {
	SetContainerLimits(
		container,
		resource.MustParse("32Mi"),
		resource.MustParse("64Mi"),
		resource.MustParse("10m"),
		resource.MustParse("100m"))
}

func SetContainerLimitsHighProfile(container *apiv1.Container) {
	SetContainerLimits(
		container,
		resource.MustParse("128Mi"),
		resource.MustParse("2Gi"),
		resource.MustParse("100m"),
		resource.MustParse("2000m"))
}

func UpdateContainerLimit(limits *v1.LimitsSpec, container *apiv1.Container) string {
	if limits != nil {
		container.Resources.Limits[apiv1.ResourceCPU] = limits.CPU
		container.Resources.Limits[apiv1.ResourceMemory] = limits.Memory
		return limits.CPU.String() + "-" + limits.Memory.String()
	}
	return ""
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

func MkEnvVarFromFieldRef(env string, fieldPath string) apiv1.EnvVar {
	return apiv1.EnvVar{
		Name: env,
		ValueFrom: &apiv1.EnvVarSource{
			FieldRef: &apiv1.ObjectFieldSelector{
				FieldPath: fieldPath,
			},
		},
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
	StorageClassName *string
	Size             resource.Quantity
	ExtraAnnotations map[string]string
}

// MkPVC produces PerssistentVolumeClaim
func MkPVC(name string, ns string, storageParams StorageConfig, accessMode apiv1.PersistentVolumeAccessMode) apiv1.PersistentVolumeClaim {
	qty := storageParams.Size
	return apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: storageParams.ExtraAnnotations,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			StorageClassName: storageParams.StorageClassName,
			AccessModes:      []apiv1.PersistentVolumeAccessMode{accessMode},
			Resources: apiv1.VolumeResourceRequirements{
				Requests: apiv1.ResourceList{
					"storage": qty,
				},
			},
		},
	}
}

// MkJob produces a Job
func MkJob(name string, ns string, container apiv1.Container, extraLabels map[string]string) batchv1.Job {
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    extraLabels,
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
				Name:       fmt.Sprintf("%s-%d", portName, p),
				Protocol:   apiv1.ProtocolTCP,
				Port:       p,
				TargetPort: intstr.FromInt(int(p)),
			})
	}
	return servicePorts
}

// MkService produces a Service
func MkService(name string, ns string, selector string, ports []int32, portName string, extraLabels map[string]string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    extraLabels,
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
func MkServicePod(name string, ns string, podName string, ports []int32, portName string, extraLabels map[string]string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    extraLabels,
		},
		Spec: apiv1.ServiceSpec{
			Ports: mkServicePorts(ports, portName),
			Selector: map[string]string{
				"statefulset.kubernetes.io/pod-name": podName,
			},
		}}

}

// MkHeadlessService produces a headless service.
func MkHeadlessService(name string, ns string, selector string, ports []int32, portName string, extraLabels map[string]string) apiv1.Service {
	service := MkService(name, ns, selector, ports, portName, extraLabels)
	service.ObjectMeta.Name = name + "-headless"
	service.Spec.ClusterIP = "None"
	return service
}

// MkHeadlessServicePod produces a headless service.
func MkHeadlessServicePod(name string, ns string, podName string, ports []int32, portName string, extraLabels map[string]string) apiv1.Service {
	service := MkServicePod(name, ns, podName, ports, portName, extraLabels)
	service.ObjectMeta.Name = name + "-headless"
	service.Spec.ClusterIP = "None"
	return service
}

// MkStatefulset produces a StatefulSet.
func MkStatefulset(
	name string, ns string, replicas int32, serviceName string,
	container apiv1.Container, pvc apiv1.PersistentVolumeClaim, extraLabels map[string]string) appsv1.StatefulSet {
	var labels = map[string]string{
		"app": "sf",
		"run": name,
	}
	maps.Copy(labels, extraLabels)
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    extraLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    utils.Int32Ptr(replicas),
			ServiceName: serviceName,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
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
func MkDeployment(name string, ns string, image string, extraLabels map[string]string) appsv1.Deployment {
	container := MkContainer(name, image)
	var labels = map[string]string{
		"app": "sf",
		"run": name,
	}
	maps.Copy(labels, extraLabels)
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: utils.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
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

func CreateHostAliases(hostAliases []v1.HostAlias) []apiv1.HostAlias {
	// Add hostAliases
	// https://kubernetes.io/docs/tasks/network/customize-hosts-file-for-pods/#adding-additional-entries-with-hostaliases
	var k8sHostAliases []apiv1.HostAlias
	for i, halias := range hostAliases {
		if hostAliases[i].IP != "" {
			k8sHostAliases = append(k8sHostAliases, apiv1.HostAlias{
				IP:        hostAliases[i].IP,
				Hostnames: halias.Hostnames,
			})
		}
	}
	return k8sHostAliases
}
