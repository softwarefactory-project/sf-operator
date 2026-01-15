# Changelog

All notable changes to this project will be documented in this file.

## [in development]

### Added

- A zuul-spare-ssh-key secret is created to help with secret rotation.

### Changed

- Force a recreation of the zookeeper TLS secrets (ca-cert, zookeeper-client-tls, zookeeper-server-tls). This fixes a typo in the code
  that prevented the client and server certs to be properly generated on a deployment that was created before v0.0.61. This is also an
  opportunity to enforce a rotation of those secrets in case such secrets were leaked due to the zuul-web issue fixed in v0.0.62.

### Deprecated
### Removed
### Fixed


## [v0.0.62] - 2026-01-09

### Changed

- Zuul is updated to version 13.1.0 to fix a critical security issue in zuul-web: https://lists.zuul-ci.org/archives/list/zuul-announce@lists.zuul-ci.org/thread/2WHXPBPRLFF6ZNSSZ3AKOBBBHMWY4YNR/


## [v0.0.61] - 2026-01-07

### Added
### Changed

- LogJuicer is updated to version 0.16.0
- The Go version in go.mod is bumped to 1.24.11. Backward compatibility with earlier versions is not guaranteed.
- Zookeeper's configuration changed to enable running as a 3-replica ensemble in a future version of sf-operator.
  For now the Zookeeper statefulset is still expected to run with just one replica; manual changes to
  the statefulset's replica count won't have any consequences (besides deploying useless pods) and will
  be canceled by sf-operator's reconciliations.
- Zookeeper: base image updated to include basic tooling to manage processes (pkill). The statefulset will be restarted to
  update the image.
- Zuul: include an unmerged patch [1] in images to improve the handling of closed zookeeper connections, especially during an executor's
  graceful stop. All components will be restarted when upgrading to this version.
- ensureDeployment function now checks if Strategy has changed.

[1]: https://review.opendev.org/c/zuul/zuul/+/967968

### Deprecated
### Removed
### Fixed

- In standalone mode, the `sf-standalone-owner` ConfigMap's `data` field is now updated on reconciliation.
- the automatic node affinity logic was failing when applied to a resource that previously had a replica count set to 0.
  Since we can't compute the node affinity in that case, skip and suggest to re-run the reconciliation in the application's
  logs.
- Logjuicer Deployment Strategy changed from RollingUpdate to Recreate.

## [v0.0.60] - 2025-11-13

### Added

- Add a `--dry-run` flag to the `deploy` command. When used, the operator will log the actions it would take without
  performing them, preventing any resource creation or modification.

### Changed

- The Go version in go.mod is bumped to 1.24.9. Backward compatibility with earlier versions is not guaranteed.
- zuul: update version to 13.0.1 https://zuul-ci.org/docs/zuul/latest/releasenotes.html#relnotes-13-0-1/
- LogJuicer is updated to version 0.15.2

### Deprecated
### Removed
### Fixed

## [v0.0.59] - 2025-10-17

### Added

- Enable storage (PVC) configuration and resizing for the `logjuicer` component.
- CLI: add the `version` subcommand, displaying the current version of the executable.
- standalone mode: the sf-standalone-owner configMap is annotated with the CLI's version that
  deployed the resource, and the end time of the deployment. The configMap's data
  is also set to hold the last applied SoftwareFactory spec.

### Changed

- The Go version in go.mod is bumped to 1.24.6. Backward compatibility with earlier versions is not guaranteed.
- The purgelogs mount point for the build logs is fixed to use the same location as the logserver.
- Zuul version bumped to 13.0.0, zuul services will be restarted during upgrade
- Zookeeper version bumped to 3.9.4
- Container images based on UBI have been updated. Most of the services will be restarted during the upgrade.
- Standarized annotations in components (gateway, git_server, hound-search, logjuicer, logserver, weeder, zookeeper, zuul)

### Deprecated
### Removed
### Fixed

- Fix a few issues with the nodeAffinity setting logic, where the node affinity of a statefulset
  would only be updated when its annotations are changed; and where it would hang if the statefulset's
  replicas are set to 0.
- Fix a race condition in the logserver controller that could cause PVCs to get stuck during operator upgrades or resource updates.

## [v0.0.58] - 2025-09-05

### Added

- zuul: the operator now validates that the user provided connections doesn't have any duplicate names.
- A new `defaultStorage.nodeAffinity` attribute that can be set to prevent pod from being scheduled to a different host and avoid issue with storage class that doesn't support that.
- zuul: increased the default executor `TerminationGracePeriodSeconds` to 2 hours and added a new CR attribute to configure the value through `Zuul.Executor.TerminationGracePeriodSeconds`.
- The operator now validates that the user-provided connections do not have any duplicate names.

### Changed

- The logjuicer service is based on a ubi9 base image
- The logjuicer service uses a persistent volume claim for data storage
- zookeeper: bumped to 3.9.3
- httpd-24: bumped to registry.access.redhat.com/ubi8/httpd-24:1-350
- weeder: update ubi9-python-39 container
- The Zuul label to run gate pipelines changed from `gateit` to `workflow`.
- Container images based on UBI have been updated. Zuul and Nodepool services will be restarted during the upgrade.
- A new platform discovery mechanism has been added to the Software Factory CLI, allowing it
  to automatically detect the Kubernetes distribution in use. As a consequence, the `OPENSHIFT_USER`
  environment variable is now optional.

### Removed

- The gitlab api token name setting is removed as it is not necessary. The value is always set to Zuul, according to https://gitlab.com/gitlab-org/gitlab/-/issues/212953

### Fixed

- Fix readiness check hanging forever if a statefulset's replica count is set to zero prior to running the "deploy" command.
- Ensure the trailing '/' when accessing `https://<domain>/logjuicer`. The web app was failing without the trailing slash.
- zuul: when the user provides a connection named `opendev.org`, the operator no longer adds its own Git connection and uses the one provided by the user for accessing zuul-jobs.
- zuul-capacity: the corporate CA certificate is now part of the CA trust chain if provided.

## [v0.0.57] - 2025-04-24

### Added

- zuul: add the `BasicAuthSecret` parameter for Elasticsearch connections. This parameter
  allows defining basic auth settings (username and password) and storing them in a secret
  rather than in plain text in the Software Factory manifest.

### Changed

- Most containers were bumped or modified to use [ubi9:latest](https://www.redhat.com/en/blog/introducing-red-hat-universal-base-image)
  as their base image, except for **git-daemon** (using ubi8) and **logjuicer** (migration still in progress).
  Incidentally, **upgrading to this version will trigger a restart of every pod in a Software Factory deployment**.
- The default CPU limits have been reduced from 2000m to 500m to enable rollout on a smaller cluster.
- The Go version in go.mod is bumped to 1.24. Backward compatibility with earlier versions is not guaranteed.
- zuul-* : bumped to 12.0.0

### Deprecated

- zuul: the `Password` parameter in SMTP connections is deprecated and will be removed
  in a future version. Use `Secrets` instead to point to a secret holding the password.

### Removed
### Fixed

- The `nodepool-sa` created using the `nodepool create openshiftpods-namespace` command now allows the creation of a port-forward so that a Zuul build running on OpenShift can succeed.

## [v0.0.56] - 2025-03-20

This release also marks the migration of the sf-operator's CI to https://gateway-cloud-softwarefactory.apps.ocp.cloud.ci.centos.org

### Added

- A new `HAS_PROC_MOUNT` environment variable is supported to deploy zuul-executor without a privileged context, providing the Kubernetes API supports the `ProcMountType` feature gate.

### Changed

- zuul-* : bumped to 11.3.0. Updated components will trigger pod rerolls at upgrade.
- Persistent Volume automatic expansion is no longer performed when the StorageClass does not support it.

### Deprecated
### Removed

- cert-manager: dependency removal complete. If you deployed a Software Factory with sf-operator prior to version
  v0.0.55, please upgrade to [v0.0.55](#v0055-2025-03-03) first to ensure the proper removal of dangling cert-manager-related resources.
- The spec `config-location` "base-url" attribute is no longer required.

### Fixed

## [v0.0.55] - 2025-03-03

🥳🎭 Carnival release 🎭🥳

### Added
### Changed

- The local certificate infrastructure (certificate authority and certificates) used by ZooKeeper, Zuul, and Zuul-Weeder
  is managed by the Golang `crypto` library instead of by the cert-manager API. The associated
  cert-manager resources will be removed, and the existing certificate infrastructure will be replaced by a newly generated
  one at upgrade time, **which will trigger a restart of the ZooKeeper, Zuul, Nodepool, and Zuul-Weeder components to update their
  respective configurations**.
  The remaining dependency on the cert-manager operator will be removed in the next release of the sf-operator.

### Deprecated
### Removed
### Fixed

## [v0.0.54] - 2025-02-26

### Added

- A new `OPENSHIFT_USER` environment variable is required to configure the operator to set up OpenShift attributes.

### Changed

- storage: `topolvm-provisioner` is no longer the default `storageClassName`; if `storageClassName` parameters
  are not set, use the cluster's default storage class for persistent volumes.

### Fixed

- The OpenShift detection logic required the permission to list CRDs; this has been changed to use a new `OPENSHIFT_USER` environment variable.

## [v0.0.53] - 2025-02-19

### Added

- hostAliases: add custom-defined hostAliases - https://kubernetes.io/docs/tasks/network/customize-hosts-file-for-pods/#adding-additional-entries-with-hostaliases

### Changed

- codesearch - re-enable the probe by using the correct endpoint `/healthz`
- codesearch - Set 4 indexers instead of 2 by default

## [v0.0.52] - 2025-02-18

### Added

- codesearch: Support for custom memory and CPU limits

### Changed

- codesearch: improved the config-update tasks
- codesearch: temporarily remove the healthcheck probe, as the service returns 503 when indexing at startup.

## [v0.0.51] - 2025-02-14

### Added

- codesearch: a new service provided by hound-search to search the repositories configured in Zuul.

### Changed

- zuul-weeder: updated the image to support the `job.role` attribute

## [v0.0.50] - 2024-12-13

### Changed

- zuul-web: add a new version (PS/13) of the upstream patch "improving UX when authn session expired" https://review.opendev.org/c/zuul/zuul/+/936440
- zuul: update version to 11.2.0 https://zuul-ci.org/docs/zuul/latest/releasenotes.html#relnotes-11-2-0

## [v0.0.49] - 2024-12-04

### Changed

- The zuul-capacity default port is changed to 9100
- zuul-web: add the upstream patch "improving UX when authn session expired" https://review.opendev.org/c/zuul/zuul/+/936440

## [v0.0.48] - 2024-11-26

### Added

- monitoring - Updated Zuul's statsd mappings to reflect metrics issued by the applications' current versions.
  This adds the following metrics:

  * zuul_tenant_pipeline_queue_branch
  * zuul_executor_pct_used_inodes
  * nodepool_image_image_build_requests
  * nodepool_builder_current_builds
  * nodepool_builder_current_uploads
  * nodepool_builder_build_workers
  * nodepool_builder_upload_workers
  * nodepool_builder_image_build_state
  * nodepool_builder_image_provider_upload_state
  * nodepool_provider_pool_addressable_requests

- development - Document the process of updating a container

### Changed

- security: bumped the cert-manager Go dependency to v1.15.4
- nodepool: update version to 10.0.0

### Deprecated
### Removed
### Fixed

## [v0.0.47] - 2024-11-20

### Fixed

- zuul-capacity service selector to enable external access

### Security

## [v0.0.46] - 2024-11-18

### Added

- zuul - add admin-rule for the internal tenant
- zuul-capacity - a service to collect cloud provider usage metrics

## [v0.0.45] - 2024-10-18

### Added
### Removed

- log forwarding - remove support for the HTTP input.

### Changed

- zuul - Increased the Zuul Scheduler and Zuul Web Startup Probes Time

### Fixed

- log forwarding - Nodepool and Zuul logs were being stamped with low precision (to the second), making verbose
  deployments hard to exploit. Logs forwarded with the `python-fluent-logger` library use the `nanosecond_precision` setting.
- logserver not restarted when `zuul-ssh-key` changes

### Security

## [v0.0.44] - 2024-10-08

### Added

- log forwarding - Added support for the [forward input](https://docs.fluentbit.io/manual/pipeline/inputs/forward).
- log forwarding - improved Fluent Bit sidecar containers' resilience to OOM killing and backpressure issues.

### Removed
### Changed


- log forwarding - The HTTP input is **deprecated**, and support for it will be removed at a later point.
- log forwarding - The predefined labels `podip`, `nodename`, and `podname` are **deprecated**.
  They are not supported in the Forward input for the Zuul and Nodepool components and will
  be removed for all components at a later point.

### Fixed

- SF bootstrap-tenant command - fix the base pre-run playbook for a container-based job (wrong condition check)

### Security

## [v0.0.43] - 2024-09-20

### Added

- crd/zuul-executor - support for `disk_limit_per_job`

### Removed
### Changed

- zookeeper - increase the certificate validity duration to 30 years

### Fixed

- zookeeper - the certificate duration bump of version v0.0.42 was partially handled due to a missing removal of the corresponding `Secrets` resources.

### Security

## [v0.0.42] - 2024-09-12

### Changed

- Zuul version has been bumped to 11.1.0 (https://zuul-ci.org/docs/zuul/latest/releasenotes.html#relnotes-11-1-0)
- zookeeper - increase the certificate validity duration to 25 years to avoid the renewal burden
- logjuicer: install corporate-ca-certs to support external SF.

### Security

- httpd-gateway: bump to the latest container image (https://catalog.redhat.com/software/containers/ubi8/httpd-24/6065b844aee24f523c207943)

## [v0.0.41] - 2024-09-11

### Added

- zuul: add corporate certs to zuul-web
- crd: add a new `branch` for the config location.

### Removed
### Changed

- zuul-scheduler: the zuul-statsd sidecar uses the default container Limit profile
- zuul-scheduler: statsd-exporter bump version to 0.27.1

### Fixed

- weeder: ensure that the service is restarted after a backup restore to load the new ZK certs.
- backup: add the logserver sshd host key `Secret` to the backup/restore process.

### Security

## [v0.0.40] - 2024-09-09

### Added

- support for storage (PVC) resize of zuul-executor, zuul-merger, and MariaDB.

## [v0.0.39] - 2024-09-04

### Added

- weeder: add the zuul-weeder service to inspect the global config, available at `$fqdn/weeder/`
- logjuicer: add a log analysis service to debug build failures, available at `$fqdn/logjuicer/`

### Fixed

- Restrict trigger rules for main branches for GitLab and Gerrit
- restore - the logserver-sshd container is not using the restored key for the authorized key

## [v0.0.38] - 2024-08-26

### Added

- add the capability to set extra annotations and extra labels for all resources
- add the capability to disable the usage of Prometheus resources
- crd: capability to disable Prometheus-related resources like `PodMonitor` to enable a `SoftwareFactory` deployment without the
  Prometheus Operator installed on the cluster.

### Changed

- crd: add a new `storageDefault` to set the default `storageClassName` and `extraAnnotations`
- crd: add new `extraLabels`

### Removed

- remove `restartPolicy` from the init container

## [v0.0.37] - 2024-08-19

### Added

- zuul: support for the SMTP connection
- zuul: added a pinned log INFO level for the "zuul.GerritConnection.ssh" logger, as it is too verbose in the DEBUG level

### Fixed

- zuul: fix services not rolling out after a log level change

## [v0.0.36] - 2024-08-14

### Added

- zuul: ensure that the `ca_certs` setting is set by default to the system CA bundle for Elasticsearch connections
- sf-operator-vuln-check: update system packages to the latest version

## [v0.0.35] - 2024-08-13

### Added

- zuul-scheduler: add support for `default_hold_expiration`.
- zuul-scheduler: add support for `max_hold_expiration`.

### Changed

- Prometheus integration: Bumped the version of statsd-exporter and node-exporter

### Fixed

- Fail to deploy Zuul when multiple connections use the same secret name (Gerrit and GitHub)

## [v0.0.34] - 2024-07-24

### Changed

- Zuul version has been bumped to 11.0.1 (https://zuul-ci.org/docs/zuul/latest/releasenotes.html#relnotes-11-0-1)

### Security

- sf-operator: Golang static code analysis test added

## [v0.0.33] - 2024-07-11

### Added

- Capability to simply run a privileged zuul-client command from any Zuul pods without providing an auth token

### Changed

- Zuul version to 11.0.0

## [v0.0.32] - 2024-07-09

### Added

- CLI - add the 'debug' flag to set the LogLevel to DEBUG
- Log Forwarding - add the preset label "labels_app" by default; its value is "sf"
- doc - how to set the Route for a deployment of Software Factory services

### Removed

- Route resource and TLS-related handling and CLI facilities have been removed

### Fixed

- zuul-merger: the CRD `logLevel` parameter is not handled correctly

### Security

- Update of the component's base container images to address several base OS security issues

## [v0.0.31] - 2024-06-06
### Fixed

- MariaDB: Stateful update to 10.6

## [v0.0.30] - 2024-06-06
### Changed

- MariaDB: change liveness and readiness probes
- MariaDB: version bumped to 10.6

## [v0.0.29] - 2024-06-03
### Added

- Default container resources (requests and limits)
- A Limits Spec in the CRD for the ZooKeeper, MariaDB, Nodepool, and Zuul components

### Changed

- Zuul: remove the SQLAlchemy logging handler, as it is too verbose in the INFO level
- Zuul: version bumped to 10.1.0

## [v0.0.28] - 2024-05-03
### Added

- CLI: restore command and documentation.
- Dev CLI - Add the command "go run main.go dev getImagesSecurityIssues" to ease getting a small report of HIGH
  and CRITICAL security issues reported by quay.io on container images used by the sf-operator.

### Changed

- ZooKeeper version bumped to 3.8.4
- The Operator handles only one Route resource, as a 'gateway' pod dispatches incoming connections.

### Removed

- The LogsServer CRD and controller, as there is no identified need for a proper CRD and controller.

### Security

- UBI9/ZooKeeper image rebuild to address reported security issues

## [v0.0.27] - 2024-03-27

🐰🔔 Easter release 🐰🔔

### Added

- "Debug" toggle for fluent bit sidecars
- Support for running the zuul-executor component external to the cluster (see ADR#014).
- The standalone deployment mode exits 1 when the reconcile is not possible after 300 seconds
- A bundled YAML file containing information about container images used by the operator: `controllers/libs/base/static/images.yaml`

### Changed

- zookeeper: update liveness and readiness probes to only check SSL access and remove the superfluous Service resource called
  `zookeeper-headless`.
- nodepool: update version to 10.0.0
- zuul: update version to 10.0.0
- CLI: simplified `SF backup` options to streamline the backup process.

### Deprecated
### Removed
### Fixed

- nodepool-builder: fixed the log path configuration when using the Fluent Bit log forwarder, resulting in much fewer file access errors appearing in Fluent Bit logs.

### Security

## [v0.0.26] - 2024-03-08

### Added

- CLI: Add the `SF backup` subcommand. This subcommand dumps a Software Factory's most important data for safekeeping.

### Changed
### Deprecated
### Removed
### Fixed
### Security

## [alpha] - not released

- Initial alpha version. Please consult the commit log for detailed information.
- From now on, all changes will be referenced in this changelog.
