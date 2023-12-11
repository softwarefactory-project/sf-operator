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
	// The fully qualified domain name to use with the log server. Logs will be served at https://`FQDN`/logs
	FQDN string `json:"fqdn"`
	// LetsEncrypt settings for enabling using LetsEncrypt for Routes/TLS
	LetsEncrypt *LetsEncryptSpec `json:"LetsEncrypt,omitempty"`
	// Default storage class to use with Persistent Volume Claims issued by this resource. Consult your cluster's configuration to see what storage classes are available and recommended for your use case.
	StorageClassName string `json:"storageClassName,omitempty"`
	// The SSH public key, encoded as base64, to use to authorize file transfers on the log server
	AuthorizedSSHKey string `json:"authorizedSSHKey"`
	// General runtime settings for the log server
	Settings LogServerSpecSettings `json:"settings,omitempty"`
}

type LogServerSpecSettings struct {
	// Logs retention time in days. Logs older than this setting in days will be purged by a pruning cronjob. Defaults to 60 days
	// +kubebuilder:default:=60
	// +kubebuilder:validation:Minimum:=1
	RetentionDays int `json:"retentionDays,omitempty"`
	// The frequency, in seconds, at which the log pruning cronjob is running. Defaults to 3600s, i.e. logs are checked for pruning every hour
	// +kubebuilder:default:=3600
	// +kubebuilder:validation:Minimum:=1
	LoopDelay int `json:"loopDelay,omitempty"`
	// Storage-related settings
	Storage StorageSpec `json:"storage,omitempty"`
}

// LogServerStatus defines the observed state of a Log server
type LogServerStatus struct {
	// The deployment status.
	Ready bool `json:"ready,omitempty"`
	// The Generation of the related Custom Resource that was last processed by the operator controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// The name of the operator handling this Custom Resource's reconciliation
	ReconciledBy string `json:"reconciledBy,omitempty"`
	// Information about ongoing or completed reconciliation processes between the Log server spec and the observed state of the cluster
	Conditions []metav1.Condition `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
//+kubebuilder:resource:shortName="logss"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

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
