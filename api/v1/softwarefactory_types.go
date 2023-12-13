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

// +kubebuilder:validation:Enum=prod;staging
// +kubebuilder:default:=staging
type LEServer string

const (
	// LEServerProd set the LetsEncrypt server to Production
	LEServerProd LEServer = "prod"

	// LEServerProd set the LetsEncrypt server to Staging
	LEServerStaging LEServer = "staging"
)

type LetsEncryptSpec struct {
	// Specify the Lets encrypt server.
	// Valid values are:
	// "staging",
	// "prod"
	Server LEServer `json:"server"`
}

type FluentBitForwarderSpec struct {
	// The Host for the Fluent Bit HTTP Input to forward logs to.
	HTTPInputHost string `json:"httpInputHost"`
	// The (optional) port on which to forward logs to, defaults to 80.
	// +kubebuilder:default:=80
	HTTPInputPort int32 `json:"httpInputPort,omitempty"`
}

type StorageSpec struct {
	// Storage space to allocate to the resource, expressed as a [Quantity](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/quantity/)
	Size resource.Quantity `json:"size"`
	// Default storage class to use with Persistent Volume Claims issued by this resource. Consult your cluster's configuration to see what storage classes are available and recommended for your use case.
	ClassName string `json:"className,omitempty"`
}

// TODO rename to ConfigRepositoryLocationSpec?

type ConfigLocationSpec struct {
	// Base URL to use to perform git-related actions on the config repository. For example, if hosted on GitHub, the Base URL would be `https://github.com/<username>/`
	// +kubebuilder:validation:Pattern:=`^https?:\/\/.+$`
	BaseURL string `json:"base-url"`
	// The name of the `config` repository. This value is appended to `base-url` to clone the repository
	// +kubebuilder:validation:MinLength:=1
	Name string `json:"name"`
	// Name of the Zuul connection through which Zuul can handle git events on the config repository
	// +kubebuilder:validation:MinLength:=1
	ZuulConnectionName string `json:"zuul-connection-name"`
}

// Describes a Zuul connection using the [gitlab driver](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#gitlab).
type GitLabConnection struct {
	// How the connection will be named in Zuul's configuration and appear in zuul-web
	Name string `json:"name"`
	// the [server](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.server)
	Server string `json:"server,omitempty"`
	// the [canonicalHostname](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.canonical_hostname)
	CanonicalHostname string `json:"canonicalHostname,omitempty"`
	// the (baseUrl)[https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.baseurl)
	BaseURL string `json:"baseUrl,omitempty"`
	// Name of the secret which containes the following keys:
	// the [api_token](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.api_token)
	// the [api_token_name](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.api_token_name)
	// the [webhook_token](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.webhook_token)
	Secrets string `json:"secrets,omitempty"`
	// the [apiTokenName](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.api_token_name)
	APITokenName string `json:"apiTokenName,omitempty"`
	// the [cloneUrl](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.cloneurl)
	CloneURL string `json:"cloneUrl,omitempty"`
	// the [keepAlive](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.keepalive)
	// +kubebuilder:validation:Minimum:=0
	KeepAlive uint8 `json:"keepAlive,omitempty"`
	// the [disableConnectionPool](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.disable_connection_pool)
	DisableConnectionPool bool `json:"disableConnectionPool,omitempty"`
}

// Describes a Zuul connection using the [github driver](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#).
type GitHubConnection struct {
	// How the connection will be named in Zuul's configuration and appear in zuul-web
	Name string `json:"name"`
	// GitHub [appID](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.app_id)
	// +optional
	AppID int `json:"appID,omitempty"`
	// Name of the secret which contains the following keys:
	// [app_key](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.app_key) must be defined if appId is defined
	// [api_token(optional)](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.api_token)
	// [webhook_token (optional)](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.webhook_token)
	// The keys must have the same name as above
	// +optional
	Secrets string `json:"secrets,omitempty"`
	// the [server](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.server)
	// +optional
	Server string `json:"server,omitempty"`
	// the [canonical_hostname](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.canonical_hostname) parameter
	// +optional
	Canonicalhostname string `json:"canonicalHostname,omitempty"`
	// the [verify_ssl](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.verify_ssl) parameter
	// +kubebuilder:default:=true
	// +optional
	VerifySSL bool `json:"verifySsl,omitempty"`
}

// Describes a Zuul connection using the [gerrit driver](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#connection-configuration)
// When an optional parameter is not specified then Zuul's defaults apply
type GerritConnection struct {
	// How the connection will be named in Zuul's configuration and appear in zuul-web
	Name string `json:"name"`
	// The gerrit server hostname. Equivalent to the [server](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.server) parameter.
	Hostname string `json:"hostname"`
	// SSH port number to the Gerrit instance. Equivalent to the [port](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.port) parameter.
	// +kubebuilder:validation:Minimum:=1024
	// +kubebuilder:validation:Maximum:=65535
	Port uint16 `json:"port,omitempty"`
	// URL to Gerrit's web interface. the [baseurl](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.baseurl) parameter.
	// +kubebuilder:validation:Pattern:=`^https?:\/\/.+$`
	Puburl string `json:"puburl,omitempty"`
	// Username that Zuul will use to authenticate on the Gerrit instance. Equivalent to the [user](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.user) parameter.
	Username string `json:"username,omitempty"`
	// The canonical hostname associated with the git repositories on the Gerrit server. Equivalent to the [canonical_hostname](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.canonical_hostname) parameter.
	Canonicalhostname string `json:"canonicalhostname,omitempty"`
	// The name of a Kubernetes secret holding the Gerrit user's API Password. The secret's data must have a key called "password". Equivalent to the [password](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.password) parameter.
	Password string `json:"password,omitempty"`
	// Set to true to force git operations over SSH even if the password attribute is set. Equivalent to the [git_over_ssh](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.git_over_ssh) parameter.
	GitOverSSH bool `json:"git-over-ssh,omitempty"`
	// Disable SSL certificate verification with the Gerrit instance when set to false. Equivalent to the [verify_ssl](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.verify_ssl) parameter.
	VerifySSL bool `json:"verifyssl,omitempty"`
}

// Describes a Zuul connection using the [git driver](https://zuul-ci.org/docs/zuul/latest/drivers/git.html#connection-configuration).
// When an optional parameter is not specified then Zuul's defaults apply
type GitConnection struct {
	// How the connection will be named in Zuul's configuration and appear in zuul-web
	Name string `json:"name"`
	// [baseurl](https://zuul-ci.org/docs/zuul/latest/drivers/git.html#attr-%3Cgit%20connection%3E.baseurl)
	// +kubebuilder:validation:Pattern:=`^(https?|git):\/\/.+$`
	Baseurl string `json:"baseurl"`
	// [poolDelay](https://zuul-ci.org/docs/zuul/latest/drivers/git.html#attr-%3Cgit%20connection%3E.poll_delays)
	PollDelay int32 `json:"pollDelay,omitempty"`
}

// Describes a Zuul connection using the [ElasticSearch driver](https://zuul-ci.org/docs/zuul/latest/drivers/elasticsearch.html#connection-configuration).
// When an optional parameter is not specified then Zuul's defaults apply
type ElasticSearchConnection struct {
	// How the connection will be named in Zuul's configuration and appear in zuul-web
	Name string `json:"name"`
	// [uri](https://zuul-ci.org/docs/zuul/latest/drivers/elasticsearch.html#attr-%3CElasticsearch%20connection%3E.uri)
	URI string `json:"uri"`
	// [useSSL](https://zuul-ci.org/docs/zuul/latest/drivers/elasticsearch.html#attr-%3CElasticsearch%20connection%3E.use_ssl)
	UseSSL *bool `json:"useSSL,omitempty"`
	// [verifyCerts](https://zuul-ci.org/docs/zuul/latest/drivers/elasticsearch.html#attr-%3CElasticsearch%20connection%3E.verify_certs)
	VerifyCerts *bool `json:"verifyCerts,omitempty"`
}

// The description of an OpenIDConnect authenticator, see [Zuul's authentication documentation](https://zuul-ci.org/docs/zuul/latest/configuration.html#authentication)
type ZuulOIDCAuthenticatorSpec struct {
	// The [name of the authenticator in Zuul's configuration](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E)
	Name string `json:"name"`
	// Authentication realm, equivalent to the [realm](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.realm) parameter
	Realm string `json:"realm"`
	// The client ID, as exposed in the `aud` claim of a JWT. Equivalent to the [client_id](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.client_id) parameter
	ClientID string `json:"clientID"`
	// The issuer ID, as exposed in the `iss` claim of a JWT. Equivalent to the [issuer_id](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.issuer_id) parameter
	IssuerID string `json:"issuerID"`
	// +kubebuilder:default:=sub
	// The JWT claim to use as a unique identifier in audit logs, equivalent to the [uid_claim](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.uid_claim) parameter
	UIDClaim string `json:"uidClaim,omitempty"`
	// Optionally override the `expires_at` claim in a JWT to enforce a custom expiration time on a token. Equivalent to the [max_validity_time](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.max_validity_time) parameter
	MaxValidityTime int32 `json:"maxValidityTime,omitempty"`
	// +kubebuilder:default:=0
	// Optionally compensate for skew between Zuul's and the Identity Provider's clocks, equivalent to the [skew](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.skew) parameter
	Skew int32 `json:"skew,omitempty"`
	// Optionally provide a URL to fetch the Identity Provider's key set, equivalent to the [keys_url](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-keys_url) parameter
	KeysURL string `json:"keysURL,omitempty"`
	// +kubebuilder:default:="openid profile"
	// The scope used to fetch a user's details, equivalent to the [scope](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-scope) parameter
	Scope string `json:"scope,omitempty"`
	// Optionally provide the claim where the authority is set if not in `iss`, equivalent to the [authority](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-authority) parameter
	Authority string `json:"authority,omitempty"`
	// Optionally provide the claim where the audience is set if not in `aud`, equivalent to the [audience](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-audience) parameter
	Audience string `json:"audience,omitempty"`
	// +kubebuilder:default:=true
	// If set to false, zuul-web will skip loading the Identity Provider's `userinfo` endpoint and rely on what's available in the JWT. Equivalent to the [load_user_info](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-load_user_info) parameter
	LoadUserInfo bool `json:"loadUserInfo,omitempty"`
}

// Spec for the pool of executor microservices
type ZuulExecutorSpec struct {
	// Storage-related settings
	Storage StorageSpec `json:"storage,omitempty"`
	// Specify the Log Level of the zuul-executor service.
	// Valid values are:
	// "INFO" (default),
	// "WARN",
	// "DEBUG".
	// Changing this value will restart the service.
	// +optional
	LogLevel LogLevel `json:"logLevel,omitempty"`
}

type ZuulWebSpec struct {
	// Specify the Log Level of the zuul-web launcher service.
	// Valid values are:
	// "INFO" (default),
	// "WARN",
	// "DEBUG".
	// Changing this value will restart the service.
	// +optional
	LogLevel LogLevel `json:"logLevel,omitempty"`
}

// Spec for the scheduler microservice
type ZuulSchedulerSpec struct {
	// Storage-related settings
	Storage StorageSpec `json:"storage,omitempty"`
	// The address to forward statsd metrics to (optional), in the form "host:port"
	StatsdTarget string `json:"statsdTarget,omitempty"`
	// Specify the Log Level of the zuul-scheduler service.
	// Valid values are:
	// "INFO" (default),
	// "WARN",
	// "DEBUG".
	// Changing this value will restart the service.
	// +optional
	LogLevel LogLevel `json:"logLevel,omitempty"`
}

// Zuul Merger Configuration, see [Zuul's documentation](https://zuul-ci.org/docs/zuul/latest/configuration.html#merger)
type ZuulMergerSpec struct {
	// the [git_user_name](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-merger.git_user_name) parameter
	// +optional
	GitUserName string `json:"gitUserName,omitempty"`
	// the [git_user_email](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-merger.git_user_email) parameter
	// +optional
	GitUserEmail string `json:"gitUserEmail,omitempty"`
	// the [git_http_low_speed_limit](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-merger.git_http_low_speed_limit) parameter
	// +kubebuilder:validation:Minimum:=0
	GitHTTPLowSpeedLimit int32 `json:"gitHttpLowSpeedLimit,omitempty"`
	// the [git_http_low_speed_time](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-merger.git_http_low_speed_time) parameter
	// +kubebuilder:validation:Minimum:=0
	GitHTTPLowSpeedTime int32 `json:"gitHttpLowSpeedTime,omitempty"`
	// the [git_timeout](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-merger.git_timeout) parameter
	// +kubebuilder:validation:Minimum:=1
	GitTimeout int32 `json:"gitTimeout,omitempty"`
	// Storage-related settings
	Storage StorageSpec `json:"storage,omitempty"`
	// Specify the Log Level of the nodepool launcher service.
	// Valid values are:
	// "INFO" (default),
	// "WARN",
	// "DEBUG".
	// Changing this value will restart the service.
	// +optional
	LogLevel LogLevel `json:"logLevel,omitempty"`
}

// TODO: make sure to update the GetConnectionsName when adding new connection type.

// Configuration of the Zuul service
type ZuulSpec struct {
	// A list of OpenID Connect authenticators that will enable admin API access on zuul-web
	OIDCAuthenticators []ZuulOIDCAuthenticatorSpec `json:"oidcAuthenticators,omitempty"`
	// The name of the default authenticator to use if no authenticator is bound explicitly to a tenant with zuul-web
	DefaultAuthenticator string `json:"defaultAuthenticator,omitempty"`
	// The list of Gerrit-based connections to add to Zuul's configuration
	GerritConns []GerritConnection `json:"gerritconns,omitempty"`
	// The list of GitHub-based connections to add to Zuul's configuration
	GitHubConns []GitHubConnection `json:"githubconns,omitempty"`
	// The list of GitLab-based connections to add to Zuul's configuration
	GitLabConns []GitLabConnection `json:"gitlabconns,omitempty"`
	// The list of Git-based connections to add to Zuul's configuration
	GitConns []GitConnection `json:"gitconns,omitempty"`
	// The list of ElasticSearch-based connections to add to Zuul's configuration
	ElasticSearchConns []ElasticSearchConnection `json:"elasticsearchconns,omitempty"`
	// Configuration of the executor microservices
	Executor ZuulExecutorSpec `json:"executor,omitempty"`
	// Configuration of the scheduler microservice
	Scheduler ZuulSchedulerSpec `json:"scheduler,omitempty"`
	// Configuration of the web microservice
	Web ZuulWebSpec `json:"web,omitempty"`
	// Configuration of the merger microservice
	Merger ZuulMergerSpec `json:"merger,omitempty"`
}

func GetGerritConnectionsName(spec *ZuulSpec) []string {
	var res []string
	res = append(res, "git-server")
	res = append(res, "opendev.org")
	for _, conn := range spec.GerritConns {
		res = append(res, conn.Name)
	}
	return res
}

func GetGitHubConnectionsName(spec *ZuulSpec) []string {
	var res []string
	for _, conn := range spec.GitHubConns {
		res = append(res, conn.Name)
	}
	return res
}

func GetGitConnectionsName(spec *ZuulSpec) []string {
	var res []string
	for _, conn := range spec.GitConns {
		res = append(res, conn.Name)
	}
	return res
}

func GetGitHubConnectionsSecretName(spec *ZuulSpec) []string {
	var res []string
	for _, conn := range spec.GitHubConns {
		res = append(res, conn.Secrets)
	}
	return res
}

func GetGitLabConnectionsName(spec *ZuulSpec) []string {
	var res []string
	for _, conn := range spec.GitLabConns {
		res = append(res, conn.Name)
	}
	return res
}

func GetGitLabConnectionsSecretName(spec *ZuulSpec) []string {
	var res []string
	for _, conn := range spec.GitLabConns {
		res = append(res, conn.Secrets)
	}
	return res
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
	// Specify the Log Level of the nodepool launcher service.
	// Valid values are:
	// "INFO" (default),
	// "WARN",
	// "DEBUG".
	// Changing this value will restart the service.
	// +optional
	LogLevel LogLevel `json:"logLevel,omitempty"`
}

type NodepoolBuilderSpec struct {
	// Storage related settings
	Storage StorageSpec `json:"storage,omitempty"`
	// Specify the Log Level of the nodepool launcher process.
	// Valid values are:
	// "INFO" (default),
	// "WARN",
	// "DEBUG".
	LogLevel LogLevel `json:"logLevel,omitempty"`
}

type NodepoolSpec struct {
	// Nodepool-launcher related settings
	Launcher NodepoolLauncherSpec `json:"launcher,omitempty"`
	// Nodepool-builder related settings
	Builder NodepoolBuilderSpec `json:"builder,omitempty"`
	// The address to forward statsd metrics to (optional), in the form "host:port"
	StatsdTarget string `json:"statsdTarget,omitempty"`
}

type ZookeeperSpec struct {
	Storage StorageSpec `json:"storage"`
}

type MariaDBSpec struct {
	// Storage parameters related to mariaDB's data
	DBStorage StorageSpec `json:"dbStorage"`
	// Storage parameters related to the database's logging
	LogStorage StorageSpec `json:"logStorage"`
}

type GitServerSpec struct {
	Storage StorageSpec `json:"storage,omitempty"`
}

type Secret struct {
	// Name of the referent.
	// More info on [kubernetes' documentation](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names).
	Name string `json:"name"`
	// The key of the secret to select from. Must be a valid secret key.
	Key string `json:"key,omitempty"`
}

type SecretRef struct {
	//Selects a key of a secret in the pod's namespace
	SecretKeyRef *Secret `json:"secretKeyRef"`
}

// SoftwareFactorySpec defines the desired state of SoftwareFactory
type SoftwareFactorySpec struct {
	// Important: Run "make manifests" to regenerate code after modifying this file

	// The fully qualified domain name to use with the deployment. Relevant services will be served
	// at https://`service`.`FQDN`
	FQDN string `json:"fqdn"`

	// LetsEncrypt settings for enabling using LetsEncrypt for Routes/TLS
	LetsEncrypt *LetsEncryptSpec `json:"letsEncrypt,omitempty"`

	// Enable log forwarding to a [Fluent Bit HTTP input](https://docs.fluentbit.io/manual/pipeline/inputs/http)
	FluentBitLogForwarding *FluentBitForwarderSpec `json:"FluentBitLogForwarding,omitempty"`

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

// TODO the exact same struct exists as `LogServerStatus`, we could merge them.

// SoftwareFactoryStatus defines the observed state of SoftwareFactory
type SoftwareFactoryStatus struct {
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
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"
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
