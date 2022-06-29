// Copied from https://review.opendev.org/c/zuul/zuul-operator/+/848103
// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package zuul

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZuulSpec defines the desired state of Zuul
type ZuulSpec struct {
	// The prefix to use for images. The image names are fixed ('zuul-executor', etc).
	ImagePrefix string `json:"imagePrefix,omitempty"`

	// If supplied, this value is passed through to Kubernetes.
	ImagePullSecrets []PullSecret `json:"imagePullSecrets,omitempty"`

	// The image tag to append to the Zuul images.
	ZuulImageVersion string `json:"zuulImageVersion,omitempty"`

	// The image tag to append to the Zuul Preview images.
	ZuulPreviewImageVersion string `json:"zuulPreviewImageVersion,omitempty"`

	// The image tag to append to the Zuul Registry images.
	ZuulRegistryImageVersion string `json:"zuulRegistryImageVersion,omitempty"`

	// The image tag to append to the Nodepool images.
	NodepoolImageVersion string `json:"nodepoolImageVersion,omitempty"`

	// This is not required unless you want to manage the database yourself.
	Database DatabaseSpec `json:"database,omitempty"`

	// This is not required unless you want to manage the ZooKeeper cluster yourself.
	ZooKeeper ZooKeeperSpec `json:"zookeeper,omitempty"`

	// A list of environment variables.
	Env map[string]string `json:"env,omitempty"`

	// The scheduler spec.
	Scheduler SchedulerSpec `json:"scheduler"`

	// The launcher spec.
	Launcher LauncherSpec `json:"launcher"`

	// The executor spec.
	Executor ExecutorSpec `json:"executor,omitempty"`

	Merger MergerSpec `json:"merger,omitempty"`
	Web WebSpec `json:"web,omitempty"`
	Finger FingerGwSpec `json:"fingergw,omitempty"`
	Preview PreviewSpec `json:"preview,omitempty"`
	Registry RegistrySpec `json:"registry,omitempty"`

	// This is a mapping designed to match the `connections` entries in the main Zuul config file.
	Connections map[string]ConnectionSpec `json:"connections"`

	// A mapping of secrets for specific Nodepool drivers.
	ExternalConfig map[string]SecretConfig `json:"externalConfig,omitempty"`

	JobVolumes []JobVolumeSpec `json:"jobVolumes,omitempty"`
}

type PullSecret struct {
	// The name of the secret.
	Name string `json:"name,omitempty"`
}

type SecretConfig struct {
	// The name of a secret.
	SecretName string `json:"secretName"`
}

type DatabaseSpec struct {
	// The name of a secret containing connection information for the database.
	// The key name in the secret should be "dburi".
	SecretName string `json:"secretName,omitempty"`

	// Only use this for testing.
	AllowUnsafeConfig bool `json:"allowUnsafeConfig,omitempty"`
}

type ZooKeeperSpec struct {
	// A standard ZooKeeper connection string.
	Hosts string `json:"hosts"`

	// The name of a secret containing a TLS client certificate and key for ZooKeeper.
	SecretName string `json:"secretName"`
}

type SchedulerSpec struct {
	Config SecretConfig `json:"config"`
}

type LauncherSpec struct {
	Config SecretConfig `json:"config"`
}

type ExecutorSpec struct {
	Count int `json:"count,omitempty"`
	SshKey SecretConfig `json:"sshkey,omitempty"`
	TerminationGracePeriodSeconds int `json:"terminationGracePeriodSeconds,omitempty"`
}

type MergerSpec struct {
	Count int `json:"count,omitempty"`
}

type WebSpec struct {
	Count int `json:"count,omitempty"`
}

type FingerGwSpec struct {
	Count int `json:"count,omitempty"`
}

type PreviewSpec struct {
	Count int `json:"count,omitempty"`
}

type RegistrySpec struct {
	Count int `json:"count,omitempty"`
	VolumeSize string `json:"volumeSize,omitempty"`
	TLS SecretConfig `json:"tls,omitempty"`
	Config SecretConfig `json:"config,omitempty"`
}

type ConnectionSpec map[string]string

type JobVolumeSpec struct {
	Context string `json:"context"`
	Access string `json:"access"`
	// The mount point within the execution context.
	Path string `json:"path"`
	// A mapping corresponding to a Kubernetes volume.
	Volume apiv1.Volume `json:"volume"`
}

// ZuulStatus defines the observed state of Zuul
type ZuulStatus struct {
	// The deployment status.
	Ready bool `json:"ready,omitempty"`
}

// Zuul is the Schema for the zuuls API
type Zuul struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZuulSpec   `json:"spec,omitempty"`
	Status ZuulStatus `json:"status,omitempty"`
}

// ZuulList contains a list of Zuul
type ZuulList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Zuul `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Zuul{}, &ZuulList{})
}
