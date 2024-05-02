# Changelog

All notable changes to this project will be documented in this file.
## [in development]

### Added

- CLI - add the 'debug' flag to set the LogLevel to DEBUG
- Log Forwarding - add preset label "labels_app" by default, its value is "sf"
- doc - how to set the Route for a deployment of Sofware Factory services

### Changed
### Deprecated
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
- Dev CLI - Add command "go run ./main.go dev getImagesSecurityIssues" to ease getting a small report of HIGH
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
