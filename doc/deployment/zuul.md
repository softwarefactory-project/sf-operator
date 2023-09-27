# Zuul

Here you will find information about managing the Zuul service when deployed with the SF Operator.
It does not replace [Zuul's documentation](https://zuul-ci.org/docs/zuul/latest/),
but addresses specificities and idiosyncrasies induced by deploying Zuul with the SF Operator.

## Table of Contents

1. [Architecture](#architecture)
1. [Services configuration](#services-configuration)
1. [Config repository](#tenant-configuration)
1. [Zuul-Client](#zuul-client)

## Architecture

Zuul is deployed by SF-Operator as micro-services:

| Service | Kubernetes resource type | Scalable Y/N |
|---------|--------------------------|-------------|
| zuul-scheduler | statefulset | N |
| zuul-executor | statefulset | Y |
| zuul-web | deployment | N |

The operator also includes backing services with bare bones support:

| Service | Kubernetes resource type | Scalable Y/N |
|---------|--------------------------|-------------|
| zookeeper | statefulset | N |
| mariadb | statefulset | N |

> For services deployed as statefulsets, it is always possible to modify the replicas amount directly in their manifests, but SF-Operator will not act upon it - for example increasing mariadb's replicas will not set up a primary node and replica nodes like a dedicated mariadb operator would. You will only end up with one node being used by Zuul, and the rest using up resources for nothing.

## Services configuration

Configuring the Zuul micro-services is done through the SoftwareFactory deployment's manifest. Many configuration parameters are exposed by The [SoftwareFactory Custom Resource spec](./../../config/crd/bases/sf.softwarefactory-project.io_softwarefactories.yaml).

The spec is constantly evolving during alpha development, and should be considered
unstable but the ultimate source of truth for documentation about its properties.

## Tenant configuration

Zuul's tenant configuration is stored in the [config repository](./config_repository.md). Edit `./zuul/main.yaml` to add, edit or delete tenants and projects on your
deployment; then commit your changes for review and CI validation.

## Zuul-Client

The `sfconfig` CLI can act as a "proxy" of sorts for the `zuul-client` CLI, by directly calling  `zuul-client` from a running Zuul web pod. For example, to read zuul-client's help message:

```bash
./tools/sfconfig zuul-client -h
```