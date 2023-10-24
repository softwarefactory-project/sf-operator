# API Reference

## Packages
- [sf.softwarefactory-project.io/v1](#sfsoftwarefactory-projectiov1)


## sf.softwarefactory-project.io/v1

Package v1 contains API Schema definitions for the sf v1 API group

### Resource Types
- [LogServer](#logserver)
- [LogServerList](#logserverlist)
- [SoftwareFactory](#softwarefactory)
- [SoftwareFactoryList](#softwarefactorylist)





#### ConfigLocationSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `base-url` _string_ | Base URL to use to perform git-related actions on the config repository. For example, if hosted on GitHub, the Base URL would be `https://github.com/<username>/` | -|
| `name` _string_ | The name of the `config` repository. This value is appended to `base-url` to clone the repository | -|
| `zuul-connection-name` _string_ | Name of the Zuul connection through which Zuul can handle git events on the config repository | -|


#### GerritConnection



Describes a Zuul connection using the [gerrit driver](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#connection-configuration)

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | How the connection will be named in Zuul's configuration and appear in zuul-web | -|
| `hostname` _string_ | The gerrit server hostname. Equivalent to the [server](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.server) parameter. | -|
| `port` _integer_ | SSH port number to the Gerrit instance. Equivalent to the [port](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.port) parameter. | {29418}|
| `puburl` _string_ | URL to Gerrit's web interface. the [baseurl](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.baseurl) parameter. | -|
| `username` _string_ | Username that Zuul will use to authenticate on the Gerrit instance. Equivalent to the [user](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.user) parameter. | {zuul}|
| `canonicalhostname` _string_ | The canonical hostname associated with the git repositories on the Gerrit server. Equivalent to the [canonical_hostname](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20connection%3E.canonical_hostname) parameter. | -|
| `password` _string_ | The name of a Kubernetes secret holding the Gerrit user's API Password. The secret's data must have a key called "password". Equivalent to the [password](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.password) parameter. | -|
| `git-over-ssh` _boolean_ | Set to true to force git operations over SSH even if the password attribute is set. Equivalent to the [git_over_ssh](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.git_over_ssh) parameter. | {false}|
| `verifyssl` _boolean_ | Disable SSL certificate verification with the Gerrit instance when set to false. Equivalent to the [verify_ssl](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#attr-%3Cgerrit%20ssh%20connection%3E.verify_ssl) parameter. | {true}|


#### GitHubConnection



Describes a Zuul connection using the [github driver](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#).

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `name` _string_ | How the connection will be named in Zuul's configuration and appear in zuul-web | -|
| `appId` _string_ | the [app_id](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.app_id) parameter | -|
| `appKey` _string_ | the [app_key](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.app_key) parameter | -|
| `apiToken` _string_ | the [api_token](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.api_token) parameter | -|
| `webHookToken` _string_ | the [webhook_token](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.webhook_token) parameter | -|
| `server` _string_ | the [server](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.server) parameter | -|
| `canonicalHostname` _string_ | the [canonical_hostname](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.canonical_hostname) parameter | -|
| `verifySsl` _boolean_ | the [verify_ssl](https://zuul-ci.org/docs/zuul/latest/drivers/github.html#attr-%3Cgithub%20connection%3E.verify_ssl) parameter | {true}|


#### GitServerSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `storage` _[StorageSpec](#storagespec)_ |  | -|


#### LEServer

_Underlying type:_ _string_



_Appears in:_
- [LetsEncryptSpec](#letsencryptspec)



#### LetsEncryptSpec





_Appears in:_
- [LogServerSpec](#logserverspec)
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `server` _[LEServer](#leserver)_ | Specify the Lets encrypt server. Valid values are: "staging", "prod" | -|


#### LogLevel

_Underlying type:_ _string_



_Appears in:_
- [NodepoolBuilderSpec](#nodepoolbuilderspec)
- [NodepoolLauncherSpec](#nodepoollauncherspec)
- [ZuulExecutorSpec](#zuulexecutorspec)
- [ZuulMergerSpec](#zuulmergerspec)
- [ZuulSchedulerSpec](#zuulschedulerspec)
- [ZuulWebSpec](#zuulwebspec)



#### LogServer



LogServer is the Schema for the LogServers API

_Appears in:_
- [LogServerList](#logserverlist)

| Field | Description | Default Value |
| --- | --- | --- |
| `apiVersion` _string_ | `sf.softwarefactory-project.io/v1` | - |
| `kind` _string_ | `LogServer` | - |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. | -|
| `spec` _[LogServerSpec](#logserverspec)_ |  | -|


#### LogServerList



LogServerList contains a list of LogServer



| Field | Description | Default Value |
| --- | --- | --- |
| `apiVersion` _string_ | `sf.softwarefactory-project.io/v1` | - |
| `kind` _string_ | `LogServerList` | - |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. | -|
| `items` _[LogServer](#logserver) array_ |  | -|


#### LogServerSpec



LogServerSpec defines the desired state of LogServer

_Appears in:_
- [LogServer](#logserver)

| Field | Description | Default Value |
| --- | --- | --- |
| `fqdn` _string_ | The fully qualified domain name to use with the log server. Logs will be served at https://logserver.`FQDN` | -|
| `LetsEncrypt` _[LetsEncryptSpec](#letsencryptspec)_ | LetsEncrypt settings for enabling using LetsEncrypt for Routes/TLS | -|
| `storageClassName` _string_ | Default storage class to use with Persistent Volume Claims issued by this resource. Consult your cluster's configuration to see what storage classes are available and recommended for your use case. | -|
| `authorizedSSHKey` _string_ | The SSH public key, encoded as base64, to use to authorize file transfers on the log server | -|
| `settings` _[LogServerSpecSettings](#logserverspecsettings)_ | General runtime settings for the log server | -|


#### LogServerSpecSettings





_Appears in:_
- [LogServerSpec](#logserverspec)
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `retentionDays` _integer_ | Logs retention time in days. Logs older than this setting in days will be purged by a pruning cronjob. Defaults to 60 days | -|
| `loopDelay` _integer_ | The frequency, in seconds, at which the log pruning cronjob is running. Defaults to 3600s, i.e. logs are checked for pruning every hour | -|
| `storage` _[StorageSpec](#storagespec)_ | Storage-related settings | -|




#### MariaDBSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `dbStorage` _[StorageSpec](#storagespec)_ | Storage parameters related to mariaDB's data | -|
| `logStorage` _[StorageSpec](#storagespec)_ | Storage parameters related to the database's logging | -|


#### NodepoolBuilderSpec





_Appears in:_
- [NodepoolSpec](#nodepoolspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `storage` _[StorageSpec](#storagespec)_ | Storage related settings | -|
| `logLevel` _[LogLevel](#loglevel)_ | Specify the Log Level of the nodepool launcher process. Valid values are: "INFO" (default), "WARN", "DEBUG". | INFO|


#### NodepoolLauncherSpec





_Appears in:_
- [NodepoolSpec](#nodepoolspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `logLevel` _[LogLevel](#loglevel)_ | Specify the Log Level of the nodepool launcher service. Valid values are: "INFO" (default), "WARN", "DEBUG". Changing this value will restart the service. | INFO|


#### NodepoolSpec





_Appears in:_
- [SoftwareFactorySpec](#softwarefactoryspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `launcher` _[NodepoolLauncherSpec](#nodepoollauncherspec)_ | Nodepool-launcher related settings | -|
| `builder` _[NodepoolBuilderSpec](#nodepoolbuilderspec)_ | Nodepool-builder related settings | -|
| `statsdTarget` _string_ | The address to forward statsd metrics to (optional), in the form "host:port" | -|


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
| `letsEncrypt` _[LetsEncryptSpec](#letsencryptspec)_ | LetsEncrypt settings for enabling using LetsEncrypt for Routes/TLS | -|
| `storageClassName` _string_ | Default storage class to use by Persistent Volume Claims | -|
| `config-location` _[ConfigLocationSpec](#configlocationspec)_ | Config repository spec | -|
| `zuul` _[ZuulSpec](#zuulspec)_ | Zuul service spec | -|
| `nodepool` _[NodepoolSpec](#nodepoolspec)_ | Nodepool services spec | -|
| `zookeeper` _[ZookeeperSpec](#zookeeperspec)_ | Zookeeper service spec | -|
| `logserver` _[LogServerSpecSettings](#logserverspecsettings)_ | Logserver service spec | -|
| `mariadb` _[MariaDBSpec](#mariadbspec)_ | MariaDB service spec | -|
| `gitserver` _[GitServerSpec](#gitserverspec)_ | Git server spec | -|




#### StorageSpec





_Appears in:_
- [GitServerSpec](#gitserverspec)
- [LogServerSpecSettings](#logserverspecsettings)
- [MariaDBSpec](#mariadbspec)
- [NodepoolBuilderSpec](#nodepoolbuilderspec)
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


#### ZuulExecutorSpec



Spec for the pool of executor microservices

_Appears in:_
- [ZuulSpec](#zuulspec)

| Field | Description | Default Value |
| --- | --- | --- |
| `storage` _[StorageSpec](#storagespec)_ | Storage-related settings | -|
| `replicas` _integer_ | How many executor pods to run | -|
| `logLevel` _[LogLevel](#loglevel)_ | Specify the Log Level of the zuul-executor service. Valid values are: "INFO" (default), "WARN", "DEBUG". Changing this value will restart the service. | INFO|


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
| `minReplicas` _integer_ | How many merger pods to run | {1}|
| `logLevel` _[LogLevel](#loglevel)_ | Specify the Log Level of the nodepool launcher service. Valid values are: "INFO" (default), "WARN", "DEBUG". Changing this value will restart the service. | INFO|


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

