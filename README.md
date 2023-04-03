# sf-operator

**sf-operator** is the next version of [Software Factory](https://www.softwarefactory-project.io).

It is an **OpenShift Operator** capable of deploying and maitaining Software Factory's services.

## Contacts

You can reach us on [the Software Factory Matrix channel](https://app.element.io/#/room/#softwarefactory-project:matrix.org).

## Status

The current project status is: **Alpha - DO NOT USE IN PRODUCTION**

See the [CONTRIBUTING documentation](CONTRIBUTING.md) to discover how to hack on the project.

Below are the milestones we are working on:

### Next milestones

TBD

### Alpha 1

- [x] Deployment of **Zuul** (scheduler, executor, web) and **Zookeeper**
- [x] Deployment of **MariaDB**
- [x] Deployment of **Gerrit**
- [x] Deployment of a **Logs Server for Zuul**
- [x] A **system-config Git repository** to host the config workflow CI configuration
- [x] A **config Git repository** to host Software Factory services configuration
- [x] A minimal **config-check** / **config-update** CI workflow
- [] Demo video

## ADR

Architecture Decision Records are available as Markdown format in *doc/adr/*.

To add a new decision:

1. Copy doc/adr/adr-template.md to doc/adr/NNNN-title-with-dashes.md, where NNNN indicates the next number in sequence.
2. Edit NNNN-title-with-dashes.md.

More information in the [ADR's README](doc/adr/README.md).
