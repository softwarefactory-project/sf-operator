// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type BaseSpec struct{}

type ConfigLocationsSpec struct {
	ConfigRepo string `json:"config-repo,omitempty"`
	User       string `json:"user,omitempty"`
}

type GerritConnection struct {
	Name              string `json:"name"`
	Hostname          string `json:"hostname"`
	Port              string `json:"port,omitempty"`
	Puburl            string `json:"puburl,omitempty"`
	Username          string `json:"username"`
	Canonicalhostname string `json:"canonicalhostname,omitempty"`
	Password          string `json:"password,omitempty"` // API Password secret name
	VerifySSL         string `json:"verifyssl,omitempty"`
}

type ZuulSpec struct {
	GerritConns []GerritConnection `json:"gerritconns,omitempty"`
}

type GerritSpec struct {
	SshdMaxConnectionsPerUser string `json:"sshd_max_connections_per_user,omitempty"`
}

type ZookeeperSpec struct {
	StorageSize string `json:"storageSize"`
}

type Secret struct {
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name"`
	// The key of the secret to select from. Must be a valid secret key.
	Key string `json:"key"`
}

type SecretRef struct {
	//Selects a key of a secret in the pod's namespace
	SecretKeyRef Secret `json:"secretKeyRef"`
}

// SoftwareFactorySpec defines the desired state of SoftwareFactory
type SoftwareFactorySpec struct {
	// Important: Run "make manifests" to regenerate code after modifying this file

	FQDN string `json:"fqdn"`

	// Default storage class to use by Persistent Volume Claims
	StorageClassName string `json:"storageClassName,omitempty"`

	// Config repositories spec
	ConfigLocations ConfigLocationsSpec `json:"config-locations,omitempty"`

	// Gerrit service spec
	Gerrit GerritSpec `json:"gerrit,omitempty"`

	// Zuul service spec
	Zuul ZuulSpec `json:"zuul,omitempty"`

	// Zookeeper service spec
	Zookeeper ZookeeperSpec `json:"zookeeper,omitempty"`
}

// SoftwareFactoryStatus defines the observed state of SoftwareFactory
type SoftwareFactoryStatus struct {
	// The deployment status.
	Ready bool `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
//+kubebuilder:resource:shortName="sf"

// SoftwareFactory is the Schema for the softwarefactories API
type SoftwareFactory struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SoftwareFactorySpec   `json:"spec,omitempty"`
	Status SoftwareFactoryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SoftwareFactoryList contains a list of SoftwareFactory
type SoftwareFactoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SoftwareFactory `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SoftwareFactory{}, &SoftwareFactoryList{})
}
