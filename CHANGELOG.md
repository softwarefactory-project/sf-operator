# Changelog

All notable changes to this project will be documented in this file.

## [in development]

### Added
### Changed
### Deprecated
### Removed
### Fixed
### Security

## [v0.0.27] - 2024-03-27

🐰🔔 Easter release 🐰🔔

### Added

- "Debug" toggle for fluent bit sidecars
- A support for running zuul-executor component external to the cluster (see ADR#014).
- The standalone deployment mode exits 1 when the reconcile is not possible after 300 seconds
- A bundled YAML file containing information about container images used by the operator [images.yaml](./controllers/libs/base/static/images.yaml)

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
