// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ConfigCheckJobSpec defines the desired state of ConfigCheckJob
type ConfigCheckJobSpec struct {
	// +kubebuilder:validation:MinLength:=1
	ZuulTenantConfig string `json:"zuulTenantConfig"`
	// +kubebuilder:default:=600
	// +kubebuilder:validation:Minimum:=60
	// +kubebuilder:validation:Maximum:=86400
	TTL int32 `json:"ttl,omitempty"`
}

// ConfigCheckJobStatus defines the observed state of ConfigCheckJob
type ConfigCheckJobStatus struct {
	Ready          bool        `json:"ready,omitempty"`
	StartTime      metav1.Time `json:"startTime,omitempty"`
	CompletionTime metav1.Time `json:"completionTime,omitempty"`
	PodID          string      `json:"podID,omitempty"`
	Outcome        string      `json:"outcome,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
//+kubebuilder:resource:shortName="ccj"

// ConfigCheckJob is the Schema for the configcheckjobs API
type ConfigCheckJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigCheckJobSpec   `json:"spec,omitempty"`
	Status ConfigCheckJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ConfigCheckJobList contains a list of ConfigCheckJob
type ConfigCheckJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigCheckJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigCheckJob{}, &ConfigCheckJobList{})
}
