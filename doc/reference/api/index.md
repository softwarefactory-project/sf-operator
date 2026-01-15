# API Reference

## Packages
- [sf.softwarefactory-project.io/v1](#sfsoftwarefactory-projectiov1)


## sf.softwarefactory-project.io/v1

Package v1 contains API Schema definitions for the sf v1 API group

### Resource Types
- [SoftwareFactory](#softwarefactory)
- [SoftwareFactoryList](#softwarefactorylist)





#### BaseStatus



BaseStatus struct which defines the observed state for a Controller Do not use this directy, it must be derived from.

_Appears in:_
- [SoftwareFactoryStatus](#softwarefactorystatus)

| Field | Description | Default Value |
| --- | --- | --- |
| `ready` _boolean_ | The deployment status. | -|
| `observedGeneration` _integer_ | The Generation of the related Custom Resource that was last processed by the operator controller | -|
| `reconciledBy` _string_ | The name of the operator handling this Custom Resource's reconciliation | -|
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#condition-v1-meta) array_ | Information about ongoing or completed reconciliation processes between the Log server spec and the observed state of the cluster | -|


#### CodesearchSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `storage` _[StorageSpec](#storagespec)_ |  | -|
| `limits` _[LimitsSpec](#limitsspec)_ | Memory/CPU Limit | {map[cpu:500m memory:2Gi]}|
| `enabled` _boolean_ | If set to false, the service won't be deployed | {true}|


#### ConfigRepositoryLocationSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | The name of the `config` repository. This value is appended to the `base-url` of the connection to clone the repository | -|
| `branch` _string_ | The branch of the `config` repository. This value is set to the load-branch. | -|
| `zuul-connection-name` _string_ | Name of the Zuul connection through which Zuul can handle git events on the config repository | -|
| `k8s-api-url` _string_ | Public URL of the k8s cluster API. This is useful when running zuul executors outside of the cluster. This is mainly used for config-update secret generation | -|
| `logserver-host` _string_ | Public HOST of the default logserver. This is useful when running zuul executors outside of the cluster. This is mainly used for config-update secret generation | -|


#### ElasticSearchConnection



Describes a Zuul connection using the [ElasticSearch driver](https://zuul-ci.org/docs/zuul/latest/drivers/elasticsearch.html#connection-configuration). When an optional parameter is not specified then Zuul's defaults apply

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | How the connection will be named in Zuul's configuration and appear in zuul-web | -|
| `uri` _string_ | [uri](https://zuul-ci.org/docs/zuul/latest/drivers/elasticsearch.html#attr-%3CElasticsearch%20connection%3E.uri) | -|
| `useSSL` _boolean_ | [useSSL](https://zuul-ci.org/docs/zuul/latest/drivers/elasticsearch.html#attr-%3CElasticsearch%20connection%3E.use_ssl) | -|
| `verifyCerts` _boolean_ | [verifyCerts](https://zuul-ci.org/docs/zuul/latest/drivers/elasticsearch.html#attr-%3CElasticsearch%20connection%3E.verify_certs) | -|
| `basicAuthSecret` _string_ | If the connection requires basic authentication, the name of the secret containing the following keys: * username * password | -|


#### FluentBitForwarderSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `debug` _boolean_ | Run fluent bit sidecars in debug mode. This will output forwarded payloads and additional info in the sidecar's logs. Defaults to false. | {false}|
| `forwardInputHost` _string_ | The Host for the Fluent Bit Forward Input to forward logs to. | -|
| `forwardInputPort` _integer_ | The (optional) port of the forward input, defaults to 24224. | {24224}|


#### GerritConnection



Describes a Zuul connection using the [gerrit driver](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#connection-configuration) When an optional parameter is not specified then Zuul's defaults apply

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | How the connection will be named in Zuul's configuration and appear in zuul-web | -|
| `hostname` _string_ | The gerrit server hostname. Equivalent to the [server](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.server) parameter. | -|
| `port` _integer_ | SSH port number to the Gerrit instance. Equivalent to the [port](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.port) parameter. | -|
| `puburl` _string_ | URL to Gerrit's web interface. the [baseurl](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.baseurl) parameter. | -|
| `username` _string_ | Username that Zuul will use to authenticate on the Gerrit instance. Equivalent to the [user](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.user) parameter. | -|
| `canonicalhostname` _string_ | The canonical hostname associated with the git repositories on the Gerrit server. Equivalent to the [canonical_hostname](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.canonical_hostname) parameter. | -|
| `password` _string_ | The name of a Kubernetes secret holding the Gerrit user's API Password. The secret's data must have a key called "password". Equivalent to the [password](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.password) parameter. | -|
| `sshkey` _string_ | The name of a Kubernetes secret holding the Gerrit user's SSH key. The secret's data must have a key called "priv". | -|
| `git-over-ssh` _boolean_ | Set to true to force git operations over SSH even if the password attribute is set. Equivalent to the [git_over_ssh](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.git_over_ssh) parameter. | -|
| `verifyssl` _boolean_ | Disable SSL certificate verification with the Gerrit instance when set to false. Equivalent to the [verify_ssl](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.verify_ssl) parameter. | -|
| `stream-events` _boolean_ | Undocumented option; if set to False this connection won't stream events; instead it will poll for merged patches every minute or so. | -|


#### GitConnection



Describes a Zuul connection using the [git driver](https://zuul-ci.org/docs/zuul/latest/drivers/git.html#connection-configuration). When an optional parameter is not specified then Zuul's defaults apply

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | How the connection will be named in Zuul's configuration and appear in zuul-web | -|
| `baseurl` _string_ | [baseurl](https://zuul-ci.org/docs/zuul/latest/drivers/git.html#attr-%3Cgit%20connection%3E.baseurl) | -|
| `pollDelay` _integer_ | [poolDelay](https://zuul-ci.org/docs/zuul/latest/drivers/git.html#attr-%3Cgit%20connection%3E.poll_delays) | -|


#### GitHubConnection



Describes a Zuul connection using the [github driver](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#).

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | How the connection will be named in Zuul's configuration and appear in zuul-web | -|
| `appID` _integer_ | GitHub [appID](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.app_id) | -|
| `secrets` _string_ | Name of the secret which contains the following keys: [app_key](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.app_key) must be defined if appId is defined [api_token(optional)](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.api_token) [webhook_token (optional)](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.webhook_token) The keys must have the same name as above | -|
| `server` _string_ | the [server](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.server) | -|
| `canonicalHostname` _string_ | the [canonical_hostname](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.canonical_hostname) parameter | -|
| `verifySsl` _boolean_ | the [verify_ssl](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.verify_ssl) parameter | {true}|


#### GitLabConnection



Describes a Zuul connection using the [gitlab driver](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#gitlab).

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | How the connection will be named in Zuul's configuration and appear in zuul-web | -|
| `server` _string_ | the [server](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.server) | -|
| `canonicalHostname` _string_ | the [canonicalHostname](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.canonical_hostname) | -|
| `baseUrl` _string_ | the (baseUrl)[https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.baseurl) | -|
| `secrets` _string_ | Name of the secret which containes the following keys: the [api_token](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.api_token) the [api_token_name](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.api_token_name) the [webhook_token](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.webhook_token) | -|
| `cloneUrl` _string_ | the [cloneUrl](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.cloneurl) | -|
| `keepAlive` _integer_ | the [keepAlive](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.keepalive) | -|
| `disableConnectionPool` _boolean_ | the [disableConnectionPool](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#attr-%3Cgitlab%20connection%3E.disable_connection_pool) | -|


#### GitServerSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `storage` _[StorageSpec](#storagespec)_ |  | -|


#### HostAlias





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `ip` _string_ |  | -|
| `hostnames` _string array_ |  | -|


#### LEServer

_Underlying type:_ _string_



_Appears in:_
- [LetsEncryptSpec](#letsencryptspec)





#### LimitsSpec





_Appears in:_
- [CodesearchSpec](#codesearchspec)
- [MariaDBSpec](#mariadbspec)
- [NodepoolBuilderSpec](#nodepoolbuilderspec)
- [NodepoolLauncherSpec](#nodepoollauncherspec)
- [ZookeeperSpec](#zookeeperspec)
- [ZuulExecutorSpec](#zuulexecutorspec)
- [ZuulMergerSpec](#zuulmergerspec)
- [ZuulSchedulerSpec](#zuulschedulerspec)
- [ZuulWebSpec](#zuulwebspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `memory` _[Quantity](https://pkg.go.dev/k8s.io/apimachinery@v0.28.2/pkg/api/resource#Quantity)_ |  | {2Gi}|
| `cpu` _[Quantity](https://pkg.go.dev/k8s.io/apimachinery@v0.28.2/pkg/api/resource#Quantity)_ |  | {500m}|


#### LogLevel

_Underlying type:_ _string_



_Appears in:_
- [NodepoolBuilderSpec](#nodepoolbuilderspec)
- [NodepoolLauncherSpec](#nodepoollauncherspec)
- [ZuulExecutorSpec](#zuulexecutorspec)
- [ZuulMergerSpec](#zuulmergerspec)
- [ZuulSchedulerSpec](#zuulschedulerspec)
- [ZuulWebSpec](#zuulwebspec)



#### LogServerSpec



LogServerSpec defines the desired state of LogServer

_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `retentionDays` _integer_ | Logs retention time in days. Logs older than this setting in days will be purged by a pruning cronjob. Defaults to 60 days | {60}|
| `loopDelay` _integer_ | The frequency, in seconds, at which the log pruning cronjob is running. Defaults to 3600s, i.e. logs are checked for pruning every hour | {3600}|
| `storage` _[StorageSpec](#storagespec)_ | Storage-related settings | -|


#### MariaDBSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `dbStorage` _[StorageSpec](#storagespec)_ | Storage parameters related to mariaDB's data | -|
| `logStorage` _[StorageSpec](#storagespec)_ | Storage parameters related to the database's logging | -|
| `limits` _[LimitsSpec](#limitsspec)_ | Memory/CPU Limit | {map[cpu:500m memory:2Gi]}|


#### NodepoolBuilderSpec





_Appears in:_
- [NodepoolSpec](#nodepoolspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `storage` _[StorageSpec](#storagespec)_ | Storage related settings | -|
| `logLevel` _[LogLevel](#loglevel)_ | Specify the Log Level of the nodepool launcher process. Valid values are: "INFO" (default), "WARN", "DEBUG". | INFO|
| `limits` _[LimitsSpec](#limitsspec)_ | Memory/CPU Limit | {map[cpu:500m memory:2Gi]}|


#### NodepoolLauncherSpec





_Appears in:_
- [NodepoolSpec](#nodepoolspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `logLevel` _[LogLevel](#loglevel)_ | Specify the Log Level of the nodepool launcher service. Valid values are: "INFO" (default), "WARN", "DEBUG". Changing this value will restart the service. | INFO|
| `limits` _[LimitsSpec](#limitsspec)_ | Memory/CPU Limit | {map[cpu:500m memory:2Gi]}|


#### NodepoolSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `launcher` _[NodepoolLauncherSpec](#nodepoollauncherspec)_ | Nodepool-launcher related settings | -|
| `builder` _[NodepoolBuilderSpec](#nodepoolbuilderspec)_ | Nodepool-builder related settings | -|
| `statsdTarget` _string_ | The address to forward statsd metrics to (optional), in the form "host:port" | -|


#### PagureConnection



Describes a Zuul connection using the [pagure driver](https://zuul-ci.org/docs/zuul/latest/drivers/pagure.html#connection-configuration).

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | How the connection will be named in Zuul's configuration and appear in zuul-web | -|
| `server` _string_ | the [server](https://zuul-ci.org/docs/zuul/latest/drivers/pagure.html#attr-<pagure connection>.server) | -|
| `canonicalHostname` _string_ | the [canonicalHostname](https://zuul-ci.org/docs/zuul/latest/drivers/pagure.html#attr-<pagure connection>.canonical_hostname) | -|
| `baseUrl` _string_ | the (baseUrl)[https://zuul-ci.org/docs/zuul/latest/drivers/pagure.html#attr-%3Cpagure%20connection%3E.baseurl) | -|
| `secrets` _string_ | Name of the secret which containes the following keys: the [api_token](https://zuul-ci.org/docs/zuul/latest/drivers/pagure.html#attr-<pagure connection>.api_token) | -|
| `appName` _string_ | the [appName](https://zuul-ci.org/docs/zuul/latest/drivers/pagure.html#attr-<pagure connection>.app_name) | -|
| `cloneUrl` _string_ | the [cloneUrl](https://zuul-ci.org/docs/zuul/latest/drivers/pagure.html#attr-<pagure connection>.cloneurl) | -|
| `sourceWhitelist` _string_ | the [sourceWhitelist](https://zuul-ci.org/docs/zuul/latest/drivers/pagure.html#attr-<pagure connection>.source_whitelist) | -|


#### SMTPConnection



Describes a Zuul connection using the [SMTP driver](https://zuul-ci.org/docs/zuul/latest/drivers/smtp.html#connection-configuration). When an optional parameter is not specified then Zuul's defaults apply

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | How the connection will be named in Zuul's configuration and appear in zuul-web | -|
| `server` _string_ | [server](https://zuul-ci.org/docs/zuul/latest/drivers/smtp.html#attr-%3Csmtp%20connection%3E.server) | -|
| `port` _integer_ | [port](https://zuul-ci.org/docs/zuul/latest/drivers/smtp.html#attr-%3Csmtp%20connection%3E.port) | -|
| `defaultFrom` _string_ | [default_from](https://zuul-ci.org/docs/zuul/latest/drivers/smtp.html#attr-%3Csmtp%20connection%3E.default_from) | -|
| `defaultTo` _string_ | [default_to](https://zuul-ci.org/docs/zuul/latest/drivers/smtp.html#attr-%3Csmtp%20connection%3E.default_to) | -|
| `user` _string_ | [user](https://zuul-ci.org/docs/zuul/latest/drivers/smtp.html#attr-%3Csmtp%20connection%3E.user) | -|
| `password` _string_ | DEPRECATED use `Secrets` instead to securely store this value [password](https://zuul-ci.org/docs/zuul/latest/drivers/smtp.html#attr-%3Csmtp%20connection%3E.password) | -|
| `tls` _boolean_ | [use_starttls](https://zuul-ci.org/docs/zuul/latest/drivers/smtp.html#attr-%3Csmtp%20connection%3E.use_starttls) | -|
| `secrets` _string_ | Name of the secret which contains the following keys: the [password](https://zuul-ci.org/docs/zuul/latest/drivers/smtp.html#attr-%3Csmtp%20connection%3E.password) | -|


#### Secret





_Appears in:_
- [SecretRef](#secretref)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | Name of the referent. More info on [kubernetes' documentation](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names). | -|
| `key` _string_ | The key of the secret to select from. Must be a valid secret key. | -|




#### SoftwareFactory



SoftwareFactory is the Schema for the softwarefactories API

_Appears in:_
- [SoftwareFactoryList](#softwarefactorylist)

| Field | Description | Default Value |
| --- | --- | --- |
| `apiVersion` _string_ | `sf.softwarefactory-project.io/v1` | - |
| `kind` _string_ | `SoftwareFactory` | - |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. | -|
| `spec` _[SoftwareFactorySpec](#softwarefactoryspec)_ |  | -|


#### SoftwareFactoryList



SoftwareFactoryList contains a list of SoftwareFactory



| Field | Description | Default Value |
| --- | --- | --- |
| `apiVersion` _string_ | `sf.softwarefactory-project.io/v1` | - |
| `kind` _string_ | `SoftwareFactoryList` | - |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. | -|
| `items` _[SoftwareFactory](#softwarefactory) array_ |  | -|


#### SoftwareFactorySpec



SoftwareFactorySpec defines the desired state of SoftwareFactory

_Appears in:_
- [SoftwareFactory](#softwarefactory)

| Field | Description | Default Value |
| --- | --- | --- |
| `fqdn` _string_ | The fully qualified domain name to use with the deployment. Relevant services will be served at https://`service`.`FQDN` | -|
| `FluentBitLogForwarding` _[FluentBitForwarderSpec](#fluentbitforwarderspec)_ | Enable log forwarding to a [Fluent Bit HTTP input](https://docs.fluentbit.io/manual/pipeline/inputs/http) | -|
| `storageDefault` _[StorageDefaultSpec](#storagedefaultspec)_ | Default setting to use by Persistent Volume Claims | -|
| `extraLabels` _object (keys:string, values:string)_ | Whether you need to add extra labels on all managed resources | -|
| `prometheusMonitorsDisabled` _boolean_ | Set to true to disable deployment of PodMonitors and related Prometheus resource | {false}|
| `config-location` _[ConfigRepositoryLocationSpec](#configrepositorylocationspec)_ | Config repository spec | -|
| `zuul` _[ZuulSpec](#zuulspec)_ | Zuul service spec | -|
| `nodepool` _[NodepoolSpec](#nodepoolspec)_ | Nodepool services spec | -|
| `zookeeper` _[ZookeeperSpec](#zookeeperspec)_ | Zookeeper service spec | -|
| `logserver` _[LogServerSpec](#logserverspec)_ | Logserver service spec | {map[loopDelay:3600 retentionDays:60]}|
| `logjuicer` _[StorageSpec](#storagespec)_ | Logjuicer service spec | -|
| `mariadb` _[MariaDBSpec](#mariadbspec)_ | MariaDB service spec | -|
| `gitserver` _[GitServerSpec](#gitserverspec)_ | Git server spec | -|
| `codesearch` _[CodesearchSpec](#codesearchspec)_ | Codesearch service spec | -|
| `hostaliases` _[HostAlias](#hostalias) array_ | HostAliases | -|




#### StandaloneZuulExecutorSpec





_Appears in:_
- [ZuulExecutorSpec](#zuulexecutorspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `controlPlanePublicZKHostname` _string_ | This is the public hostname or IP where control plane's Zookeeper can be reached | -|
| `controlPlanePublicZKHostnames` _string_ | This is the public hostnames or IPs where control plane's Zookeepers can be reached | -|
| `controlPlanePublicGSHostname` _string_ | This is the public hostname or IP where control plane's GitServer can be reached | -|
| `publicHostname` _string_ | This is the public host or IP address reachable from zuul-web | -|


#### StorageDefaultSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `className` _string_ | Default storage class to use with Persistent Volume Claims issued by this resource. Consult your cluster's configuration to see what storage classes are available and recommended for your use case. | -|
| `extraAnnotations` _object (keys:string, values:string)_ | Whether you need to add extra annotations to the Persistent Volume Claims | -|
| `nodeAffinity` _boolean_ | Set node affinity to prevent the pod from being scheduled to a different host when using custom storage class which doesn't work nicely in that case, e.g. iSCSI | -|


#### StorageSpec





_Appears in:_
- [CodesearchSpec](#codesearchspec)
- [GitServerSpec](#gitserverspec)
- [LogServerSpec](#logserverspec)
- [MariaDBSpec](#mariadbspec)
- [NodepoolBuilderSpec](#nodepoolbuilderspec)
- [SoftwareFactorySpec](#softwarefactoryspec)
- [ZookeeperSpec](#zookeeperspec)
- [ZuulExecutorSpec](#zuulexecutorspec)
- [ZuulMergerSpec](#zuulmergerspec)
- [ZuulSchedulerSpec](#zuulschedulerspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `size` _[Quantity](https://pkg.go.dev/k8s.io/apimachinery@v0.28.2/pkg/api/resource#Quantity)_ | Storage space to allocate to the resource, expressed as a [Quantity](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/quantity/) | -|
| `className` _string_ | Default storage class to use with Persistent Volume Claims issued by this resource. Consult your cluster's configuration to see what storage classes are available and recommended for your use case. | -|


#### ZookeeperSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `storage` _[StorageSpec](#storagespec)_ |  | -|
| `limits` _[LimitsSpec](#limitsspec)_ | Memory/CPU Limit | {map[cpu:500m memory:2Gi]}|


#### ZuulExecutorSpec



Spec for the pool of executor microservices

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `storage` _[StorageSpec](#storagespec)_ | Storage-related settings | -|
| `logLevel` _[LogLevel](#loglevel)_ | Specify the Log Level of the zuul-executor service. Valid values are: "INFO" (default), "WARN", "DEBUG". Changing this value will restart the service. | INFO|
| `enabled` _boolean_ | If set to false, the zuul-executor deployment won't be applied | {true}|
| `standalone` _[StandaloneZuulExecutorSpec](#standalonezuulexecutorspec)_ | When set the Control plane is not deployed. The standalone executor must be able to connect to the control plane | -|
| `limits` _[LimitsSpec](#limitsspec)_ | Memory/CPU Limit | {map[cpu:500m memory:2Gi]}|
| `diskLimitPerJob` _integer_ | the [disk_limit_per_job](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-executor.disk_limit_per_job) | {250}|
| `TerminationGracePeriodSeconds` _integer_ |  | {7200}|


#### ZuulMergerSpec



Zuul Merger Configuration, see [Zuul's documentation](https://zuul-ci.org/docs/zuul/latest/configuration.html#merger)

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `gitUserName` _string_ | the [git_user_name](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-merger.git_user_name) parameter | -|
| `gitUserEmail` _string_ | the [git_user_email](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-merger.git_user_email) parameter | -|
| `gitHttpLowSpeedLimit` _integer_ | the [git_http_low_speed_limit](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-merger.git_http_low_speed_limit) parameter | -|
| `gitHttpLowSpeedTime` _integer_ | the [git_http_low_speed_time](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-merger.git_http_low_speed_time) parameter | -|
| `gitTimeout` _integer_ | the [git_timeout](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-merger.git_timeout) parameter | -|
| `storage` _[StorageSpec](#storagespec)_ | Storage-related settings | -|
| `logLevel` _[LogLevel](#loglevel)_ | Specify the Log Level of the nodepool launcher service. Valid values are: "INFO" (default), "WARN", "DEBUG". Changing this value will restart the service. | INFO|
| `limits` _[LimitsSpec](#limitsspec)_ | Memory/CPU Limit | {map[cpu:500m memory:2Gi]}|


#### ZuulOIDCAuthenticatorSpec



The description of an OpenIDConnect authenticator, see [Zuul's authentication documentation](https://zuul-ci.org/docs/zuul/latest/configuration.html#authentication)

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | The [name of the authenticator in Zuul's configuration](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E) | -|
| `realm` _string_ | Authentication realm, equivalent to the [realm](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.realm) parameter | -|
| `clientID` _string_ | The client ID, as exposed in the `aud` claim of a JWT. Equivalent to the [client_id](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.client_id) parameter | -|
| `issuerID` _string_ | The issuer ID, as exposed in the `iss` claim of a JWT. Equivalent to the [issuer_id](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.issuer_id) parameter | -|
| `uidClaim` _string_ | The JWT claim to use as a unique identifier in audit logs, equivalent to the [uid_claim](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.uid_claim) parameter | {sub}|
| `maxValidityTime` _integer_ | Optionally override the `expires_at` claim in a JWT to enforce a custom expiration time on a token. Equivalent to the [max_validity_time](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.max_validity_time) parameter | -|
| `skew` _integer_ | Optionally compensate for skew between Zuul's and the Identity Provider's clocks, equivalent to the [skew](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-auth%20%3Cauthenticator%20name%3E.skew) parameter | {0}|
| `keysURL` _string_ | Optionally provide a URL to fetch the Identity Provider's key set, equivalent to the [keys_url](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-keys_url) parameter | -|
| `scope` _string_ | The scope used to fetch a user's details, equivalent to the [scope](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-scope) parameter | {openid profile}|
| `authority` _string_ | Optionally provide the claim where the authority is set if not in `iss`, equivalent to the [authority](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-authority) parameter | -|
| `audience` _string_ | Optionally provide the claim where the audience is set if not in `aud`, equivalent to the [audience](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-audience) parameter | -|
| `loadUserInfo` _boolean_ | If set to false, zuul-web will skip loading the Identity Provider's `userinfo` endpoint and rely on what's available in the JWT. Equivalent to the [load_user_info](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-load_user_info) parameter | {true}|


#### ZuulSchedulerSpec



Spec for the scheduler microservice

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `storage` _[StorageSpec](#storagespec)_ | Storage-related settings | -|
| `statsdTarget` _string_ | The address to forward statsd metrics to (optional), in the form "host:port" | -|
| `logLevel` _[LogLevel](#loglevel)_ | Specify the Log Level of the zuul-scheduler service. Valid values are: "INFO" (default), "WARN", "DEBUG". Changing this value will restart the service. | INFO|
| `limits` _[LimitsSpec](#limitsspec)_ | Memory/CPU Limit | {map[cpu:500m memory:2Gi]}|
| `DefaultHoldExpiration` _integer_ | the [DefaultHoldExpiration](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-scheduler.default_hold_expiration) | -|
| `MaxHoldExpiration` _integer_ | the [MaxHoldExpiration](https://zuul-ci.org/docs/zuul/latest/configuration.html#attr-scheduler.max_hold_expiration) | -|


#### ZuulSpec



Configuration of the Zuul service

_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `oidcAuthenticators` _[ZuulOIDCAuthenticatorSpec](#zuuloidcauthenticatorspec) array_ | A list of OpenID Connect authenticators that will enable admin API access on zuul-web | -|
| `defaultAuthenticator` _string_ | The name of the default authenticator to use if no authenticator is bound explicitly to a tenant with zuul-web | -|
| `gerritconns` _[GerritConnection](#gerritconnection) array_ | The list of Gerrit-based connections to add to Zuul's configuration | -|
| `githubconns` _[GitHubConnection](#githubconnection) array_ | The list of GitHub-based connections to add to Zuul's configuration | -|
| `gitlabconns` _[GitLabConnection](#gitlabconnection) array_ | The list of GitLab-based connections to add to Zuul's configuration | -|
| `gitconns` _[GitConnection](#gitconnection) array_ | The list of Git-based connections to add to Zuul's configuration | -|
| `pagureconns` _[PagureConnection](#pagureconnection) array_ | The list of Pagure-based connections to add to Zuul's configuration | -|
| `elasticsearchconns` _[ElasticSearchConnection](#elasticsearchconnection) array_ | The list of ElasticSearch-based connections to add to Zuul's configuration | -|
| `smtpconns` _[SMTPConnection](#smtpconnection) array_ | The list of SMTP-based connections to add to Zuul's configuration | -|
| `executor` _[ZuulExecutorSpec](#zuulexecutorspec)_ | Configuration of the executor microservices | -|
| `scheduler` _[ZuulSchedulerSpec](#zuulschedulerspec)_ | Configuration of the scheduler microservice | -|
| `web` _[ZuulWebSpec](#zuulwebspec)_ | Configuration of the web microservice | -|
| `merger` _[ZuulMergerSpec](#zuulmergerspec)_ | Configuration of the merger microservice | -|


#### ZuulWebSpec





_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `logLevel` _[LogLevel](#loglevel)_ | Specify the Log Level of the zuul-web launcher service. Valid values are: "INFO" (default), "WARN", "DEBUG". Changing this value will restart the service. | INFO|
| `limits` _[LimitsSpec](#limitsspec)_ | Memory/CPU Limit | {map[cpu:500m memory:2Gi]}|



#### SoftwareFactoryStatus

_Type alias for:_ _[BaseStatus](#basestatus)_

SoftwareFactoryStatus defines the observed state of SoftwareFactory. It is a type alias for BaseStatus with no additional fields.

_Appears in:_
- [SoftwareFactory](#softwarefactory)

#### SecretRef

_This type is defined but not currently used in the public API._

SecretRef selects a key of a secret in the pod's namespace.

_Appears in:_
- [Secret](#secret)

| Field | Description | Default Value |
| --- | --- | --- |
| `secretKeyRef` _[Secret](#secret)_ | Selects a key of a secret in the pod's namespace | - |

#### LetsEncryptSpec

_This type is defined but not currently implemented in the operator._

LetsEncryptSpec specifies the Let's Encrypt server configuration. This feature is planned but not yet implemented.

| Field | Description | Default Value |
| --- | --- | --- |
| `server` _[LEServer](#leserver)_ | Specify the Let's Encrypt server. Valid values are: "staging", "prod" | staging |

