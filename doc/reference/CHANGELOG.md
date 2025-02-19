# Changelog

All notable changes to this project will be documented in this file.

## [in development]

### Added
### Changed
### Deprecated
### Removed
### Fixed

## [v0.0.53] - 2025-02-19

### Added

- hostAliases: add custom defined hostAliases - https://kubernetes.io/docs/tasks/network/customize-hosts-file-for-pods/#adding-additional-entries-with-hostaliases

### Changed

- codesearch - re-enable the probe by using the correct endpoint "/healthz"
- codesearch - Set 4 indexers instead of 2 by default

## [v0.0.52] - 2025-02-18

### Added

- codesearch: Support for custom Memory and CPU limits

### Changed

- codesearch: improved the config-update tasks
- codesearch: temporary remove the healthcheck probe as the service returns 503 when indexing, at startup.

## [v0.0.51] - 2025-02-14

### Added

- codesearch: a new service provided by hound-search to search the repositories configured in Zuul.

### Changed

- zuul-weeder: updated the image to support job.role attribute

## [v0.0.50] - 2024-12-13

### Changed

- zuul-web: add new version (PS/13) of the upstream patch "improving UX when authn session expired" https://review.opendev.org/c/zuul/zuul/+/936440
- zuul: update version to 11.2.0 https://zuul-ci.org/docs/zuul/latest/releasenotes.html#relnotes-11-2-0

## [v0.0.49] - 2024-12-04

### Changed

- zuul-capacity default port is changed to 9100
- zuul-web: add upstream patch "improving UX when authn session expired" https://review.opendev.org/c/zuul/zuul/+/936440

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

- development - Document the process to update a container

### Changed

- security: bumped cert-manager go dependency to v1.15.4
- nodepool: update version to 11.0.0

### Deprecated
### Removed
### Fixed

## [v0.0.47] - 2024-11-20

### Fixed

- zuul-capacity service selector to enable external access

### Security

## [v0.0.46] - 2024-11-18

### Added

- zuul - add admin-rule for internal tenant
- zuul-capacity - a service to collect cloud provider usage metrics

## [v0.0.45] - 2024-10-18

### Added
### Removed

- log forwarding - remove support for the HTTP input.

### Changed

- zuul - Increased Zuul Scheduler and Zuul Web Startup Probes Time

### Fixed

- log forwarding - nodepool and zuul logs were being stamped with a low precision (to the second), making verbose
  deployments hard to exploit. Logs forwarded with the python-fluent-logger library use the "nanosecond_precision" setting.
- logserver not restarted when zuul-ssh-key change

### Security

## [v0.0.44] - 2024-10-08

### Added

- log forwarding - Added support for the [forward input](https://docs.fluentbit.io/manual/pipeline/inputs/forward).
- log forwarding - improved fluent bit sidecar containers' resilience to OOM killing and backpressure issues.

### Removed
### Changed


- log forwarding - The HTTP input is **deprecated** and support for it will be removed at a later point.
- log forwarding - The predefined labels `podip`, `nodename` and `podname` are **deprecated**.
  They are not supported in the Forward input for the Zuul and Nodepool components, and will
  be removed for all components at a later point.

### Fixed

- SF bootstrap-tenant command - fix the base pre-run playbook for container based job (wrong condition check)

### Security

## [v0.0.43] - 2024-09-20

### Added

- crd/zuul-executor - support for disk_limit_per_job

### Removed
### Changed

- zookeeper - increase certificate validity duration to 30 years

### Fixed

- zookeeper - certificates duration bump of version v0.0.42 was partially handled due to a missing removal of the corresponding `Secrets` resources.

### Security

## [v0.0.42] - 2024-09-12

### Changed

- Zuul version has been bumped to 11.1.0 (https://zuul-ci.org/docs/zuul/latest/releasenotes.html#relnotes-11-1-0)
- zookeeper - increase certificate validity duration to 25 years to avoid renewal burden
- logjuicer: install corporate-ca-certs to support external SF.

### Security

- httpd-gateway: bump to latest container image (https://catalog.redhat.com/software/containers/ubi8/httpd-24/6065b844aee24f523c207943)

## [v0.0.41] - 2024-09-11

### Added

- zuul: add corporate certs to zuul-web
- crd: add new `branch` for the config location.

### Removed
### Changed

- zuul-scheduler: zuul-statsd sidecar use default container Limit profile
- zuul-scheduler: statds-exporter bump version to 0.27.1

### Fixed

- weeder: ensure the service is restarted after backup restore to load the new ZK certs.
- backup: add the logserver sshd host key `Secret` into the backup/restore process.

### Security

## [v0.0.40] - 2024-09-09

### Added

- support for storage (PVC) resize of zuul-executor, zuul-merger and mariadb.

## [v0.0.39] - 2024-09-04

### Added

- weeder: add zuul-weeder service to inspect the global config, available at $fqdn/weeder/
- logjuicer: add log analysis service to debug build failure, available at $fqdn/logjuicer/

### Fixed

- Restrict trigger rules for main branches for gitlab and gerrit
- restore - logserver-sshd container not using the restored key for the authorized key

## [v0.0.38] - 2024-08-26

### Added

- add capability to set extra annotations and extra Labels for all resources
- add capability to disable usage of Prometheus resources
- crd: capability disable Prometheus related resources like `PodMonitor` to enable `SoftwareFactory` deployment without
  Prometheus Operator installed on the cluster.

### Changed

- crd: add new `storageDefault` to set default `storageClassName` and `extraAnnotations`
- crd: add new `extraLabels`

### Removed

- remove restartPolicy from init container

## [v0.0.37] - 2024-08-19

### Added

- zuul: support of the SMTP connection
- zuul: added pinned log INFO level for "zuul.GerritConnection.ssh" logger as too verbose in DEBUG level

### Fixed

- zuul: fix services not rollout after log level change

## [v0.0.36] - 2024-08-14

### Added

- zuul: ensure ca_certs setting is set by default to system CA bundle for elasticsearch connections
- sf-operator-vuln-check: update system packages to last version

## [v0.0.35] - 2024-08-13

### Added

- zuul-scheduler: add support for `default_hold_expiration`.
- zuul-scheduler: add support for `max_hold_expiration`.

### Changed

- prometheus integration: Bumped version of statsd-exporter and node-exporter

### Fixed

- Fail to deploy Zuul when multiple connections use the same Secret name (Gerrit and Github)

## [v0.0.34] - 2024-07-24

### Changed

- Zuul version has been bumped to 11.0.1 (https://zuul-ci.org/docs/zuul/latest/releasenotes.html#relnotes-11-0-1)

### Security

- sf-operator: Golang static code analysis test added

## [v0.0.33] - 2024-07-11

### Added

- Capability to simply run privileged zuul-client command from any zuul pods without providing an auth token

### Changed

- Zuul version to 11.0.0

## [v0.0.32] - 2024-07-09

### Added

- CLI - add the 'debug' flag to set the LogLevel to DEBUG
- Log Forwarding - add preset label "labels_app" by default, its value is "sf"
- doc - how to set the Route for a deployment of Sofware Factory services

### Removed

- Route resource and TLS related handling and CLI facilities has been removed

### Fixed

- zuul-merger: CRD logLevel parameter not handled correctly

### Security

- Update of components base container images to addess several base OS security issues

## [v0.0.31] - 2024-06-06
### Fixed

- MariaDB: Statefull update to 10.6

## [v0.0.30] - 2024-06-06
### Changed

- MariaDB: change liveness and readiness probes
- MariaDB: version bumped to 10.6

## [v0.0.29] - 2024-06-03
### Added

- Default containers resources (requests and limits)
- A Limits Spec in the CRD for Zookeeper, MariaDB, Nodepool and Zuul components

### Changed

- Zuul: remove sqlalchemy logging handler as too verbose in INFO level
- Zuul: version bumped to 10.1.0

## [v0.0.28] - 2024-05-03
### Added

- CLI: restore command and documentation.
- Dev CLI - Add command "go run main.go dev getImagesSecurityIssues" to ease getting a small report of HIGH
  and CRITICAL Security issues reported by quay.io on container images used by the sf-operator.

### Changed

- Zookeeper version bumped to 3.8.4
- The Operator handles only one Route resource as a 'gateway' pod dispatches incoming connections.

### Removed

- The LogsServer CRD and controller. As there is no identified need for a proper CRD and Controller.

### Security

- UBI9/Zookeeper image rebuid to address reported security issues

## [v0.0.27] - 2024-03-27

🐰🔔 Easter release 🐰🔔

### Added

- "Debug" toggle for fluent bit sidecars
- A support for running zuul-executor component external to the cluster (see ADR#014).
- The standalone deployment mode exits 1 when the reconcile is not possible after 300 seconds
- A bundled YAML file containing information about container images used by the operator `controllers/libs/base/static/images.yaml`

### Changed

- zookeeper: update liveness and readyness probes to only check SSL access and remove superfluous Service resource called
  zookeeper-headless.
- nodepool: update version to 10.0.0
- zuul: update version to 10.0.0
- CLI: simplified `SF backup` options to streamline the backup process.

### Deprecated
### Removed
### Fixed

- nodepool-builder: fixed the log path configuration when using the fluent bit log forwarder, resulting in much less file access errors appearing in fluent bit logs.

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
- From now on all changes will be referenced into this changelog.
