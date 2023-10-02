# Changelog

All notable changes to this project will be documented in this file.

- [X] Zuul: Add OIDC Authenticators section in spec

## [Alpha-2]

- [X] Add Zuul Log Levels to CRD
- [X] Add Zuul Bootstrap Zuul Tenant Config subcommand to sfconfig cli
- [X] Create ADR for sfconfig binary
- [X] issue disk space metrics	Matthieu Huin
- [X] Add a new Zuul connection (external Gerrit)
- [X] Continuous delivery via OLM
- [X] Enable Ansible linter in sf-operator
- [X] Remove managesf support + extract gerrit
- [X] Enable Zuul Executor scaling
- [] Add Operator CLI in sf-config based on adr/0005-ops-tooling.md
- [] Support for config-check (Zuul config)
- [] Enable nodepool-launcher config via config-check and config-update
- [] nodepool-launcher: ensure nodepool-launcher can create and use containers on microshift for jobs

## [Alpha-1]

- [X] Ensure all sf-operator containers are based on stream9-minimal
- [X] Ensure Zuul live console feature via functional tests
- [X] Fix SecurityContextConstraint violations
- [X] Add purge logs container to logserver pod
- [X] Provide a capability to set PVC size for component’s volumes
- [X] Logserver lifecycle: scaling the data volume

## [MVP]

- [X] Deployment of **Zuul** (scheduler, executor, web) and **Zookeeper**
- [X] Deployment of **MariaDB**
- [X] Deployment of **Gerrit**
- [X] Deployment of a **Logs Server for Zuul**
- [X] A **system-config Git repository** to host the config workflow CI configuration
- [X] A **config Git repository** to host Software Factory services configuration
- [X] A minimal **config-check** / **config-update** CI workflow
