# Changelog

All notable changes to this project will be documented in this file.

## [Alpha-1]

- [X] Ensure all sf-operator containers are based on stream9-minimal
- [X] Ensure Zuul live console feature via functional tests
- [X] Fix SecurityContextConstraint violations
- [] Add purge logs container to logserver pod
- [] Provide a capability to set PVC size for component’s volumes
- [] Logserver lifecycle: scaling the data volume

## [MVP] - 2023-03-31 (Not released)

- Deployment of **Zuul** (scheduler, executor, web) and **Zookeeper**
- Deployment of **MariaDB**
- Deployment of **Gerrit**
- Deployment of a **Logs Server for Zuul**
- A **system-config Git repository** to host the config workflow CI configuration
- A **config Git repository** to host Software Factory services configuration
- A minimal **config-check** / **config-update** CI workflow