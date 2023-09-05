// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LogServerSpec defines the desired state of LogServer
type LogServerSpec struct {
	FQDN string `json:"fqdn"`
	// LetsEncrypt settings for enabling using LetsEncrypt for Routes/TLS
	LetsEncrypt *LetsEncryptSpec `json:"LetsEncrypt,omitempty"`
	// Default storage class to use by Persistent Volume Claims
	StorageClassName string `json:"storageClassName,omitempty"`
	// SSH authorized key as base64 data
	AuthorizedSSHKey string                `json:"authorizedSSHKey"`
	Settings         LogServerSpecSettings `json:"settings,omitempty"`
}

// +kubebuilder:validation:Required
// +kubebuilder:validation:XValidation:rule="!has(self.retentionDays) || self.retentionDays > 0",message="retentionDays must be a positive integer if set"
// +kubebuilder:validation:XValidation:rule="!has(self.loopDelay) || self.loopDelay > 0",message="loopDelay must be a positive integer if set"
type LogServerSpecSettings struct {
	// Logs Older that "x" days will be purge ( default 60 days )
	// +optional
	RetentionDays int `json:"retentionDays,omitempty"`
	// Logs Check. Log will be checked every "X" seconds ( default 3600 s ~= 1 hour )
	// +optional
	LoopDelay int         `json:"loopDelay,omitempty"`
	Storage   StorageSpec `json:"storage,omitempty"`
}

// LogServerStatus defines the observed state of LogServer
type LogServerStatus struct {
	// The deployment status.
	Ready              bool   `json:"ready,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
	ReconciledBy       string `json:"reconciledBy,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
//+kubebuilder:resource:shortName="logss"

// LogServer is the Schema for the LogServers API
type LogServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LogServerSpec   `json:"spec,omitempty"`
	Status LogServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LogServerList contains a list of LogServer
type LogServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogServer{}, &LogServerList{})
}
