// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type BaseSpec struct{}

type StorageSpec struct {
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:XValidation:rule="self >= oldSelf",message="Storage shrinking is not supported"
	Size      resource.Quantity `json:"size,string"`
	ClassName string            `json:"className,omitempty"`
}

type ConfigLocationSpec struct {
	// Base URL of the code-review provider where the `Name` can be fetch by Git
	// +kubebuilder:validation:Pattern:=`^https?:\/\/.+$`
	BaseURL string `json:"base-url"`
	// Name of the `config` repository where the SF config workflow is applied
	// +kubebuilder:validation:MinLength:=1
	Name string `json:"name"`
	// Name of the Zuul connection where the `config` exists
	// +kubebuilder:validation:MinLength:=1
	ZuulConnectionName string `json:"zuul-connection-name"`
}

type GerritConnection struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	// SSH port number to connect on the Gerrit instance
	// +kubebuilder:default:=29418
	Port uint16 `json:"port,omitempty"`
	// URL to Gerrit web interface
	// +kubebuilder:validation:Pattern:=`^https?:\/\/.+$`
	Puburl string `json:"puburl,omitempty"`
	// Username to authenticate on the Gerrit instance
	// +kubebuilder:default:=zuul
	Username string `json:"username,omitempty"`
	// The canonical hostname associated with the git repos on the Gerrit server.
	Canonicalhostname string `json:"canonicalhostname,omitempty"`
	// API Password secret name
	Password string `json:"password,omitempty"`
	// This forces git operation over SSH even if the password attribute is set.
	// +kubebuilder:default:=false
	GitOverSSH bool `json:"git-over-ssh,omitempty"`
	// Disable SSL certificate verification when set to false.
	// +kubebuilder:default:=true
	VerifySSL bool `json:"verifyssl,omitempty"`
}

type ZuulExecutorSpec struct {
	Storage  StorageSpec `json:"storage,omitempty"`
	Replicas int32       `json:"replicas,omitempty"`
}

type ZuulSchedulerSpec struct {
	Storage StorageSpec `json:"storage,omitempty"`
}

// TODO should be ExecutorS / SchedulerS
type ZuulSpec struct {
	GerritConns []GerritConnection `json:"gerritconns,omitempty"`
	Executor    ZuulExecutorSpec   `json:"executor,omitempty"`
	Scheduler   ZuulSchedulerSpec  `json:"scheduler,omitempty"`
}

// +kubebuilder:validation:Enum=INFO;WARN;DEBUG
// +kubebuilder:default:=INFO
type LogLevel string

const (
	// InfoLogLevel set log level to INFO
	InfoLogLevel LogLevel = "INFO"

	// WarnLogLevel set log level to WARN
	WarnLogLevel LogLevel = "WARN"

	// DebugLogLevel set log level to DEBUG
	DebugLogLevel LogLevel = "DEBUG"
)

type NodepoolLauncherSpec struct {
	// Specify the Log Level of the nodepool launcher process.
	// Valid values are:
	// - "INFO" (default)
	// - "WARN"
	// - "DEBUG"
	// +optional
	LogLevel LogLevel `json:"logLevel,omitempty"`
}

type NodepoolSpec struct {
	Launcher NodepoolLauncherSpec `json:"launcher,omitempty"`
}

type ZookeeperSpec struct {
	Storage StorageSpec `json:"storage"`
}

type MariaDBSpec struct {
	DBStorage  StorageSpec `json:"dbStorage"`
	LogStorage StorageSpec `json:"logStorage"`
}

type GitServerSpec struct {
	Storage StorageSpec `json:"storage,omitempty"`
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

	// Config repository spec
	ConfigLocation ConfigLocationSpec `json:"config-location,omitempty"`

	// Zuul service spec
	Zuul ZuulSpec `json:"zuul,omitempty"`

	// Nodepool services spec
	Nodepool NodepoolSpec `json:"nodepool,omitempty"`

	// Zookeeper service spec
	Zookeeper ZookeeperSpec `json:"zookeeper,omitempty"`

	// Logserver service spec
	Logserver LogServerSpecSettings `json:"logserver,omitempty"`

	// MariaDB service spec
	MariaDB MariaDBSpec `json:"mariadb,omitempty"`

	// Git server spec
	GitServer GitServerSpec `json:"gitserver,omitempty"`
}

// SoftwareFactoryStatus defines the observed state of SoftwareFactory
type SoftwareFactoryStatus struct {
	// The deployment status.
	Ready              bool   `json:"ready,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
	ReconciledBy       string `json:"reconciledBy,omitempty"`
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
