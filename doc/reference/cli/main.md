# CLI

The operator's `main.go` can be used to perform various actions related to the management of Software Factory
deployments, beyond what can be defined in a custom resource manifest.

## Table of Contents

1. [Running the CLI](#running-the-cli)
1. [Global Flags](#global-flags)
1. [Configuration File](#configuration-file)
1. [Subcommands](#subcommands)
  1. [Apply](#apply)
  1. [Operator](#apply)
  1. [Backup](#backup)
  1. [Restore](#restore)

## Running the CLI

To run the CLI, assuming you are at the root directory of the sf-operator repository:

```sh
go run ./main.go [GLOBAL FLAGS] [SUBCOMMAND] [SUBCOMMAND FLAGS] ...
```

## Global Flags

These flags apply 

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
|-n, --namespace |string | The namespace on which to perform actions | Dependent |-|
|-d, --fqdn | string | The FQDN of the deployment (if no manifest is provided) | Yes | sfop.me |
|-C, --config | string | Path to the CLI configuration file | Yes | - |
|-c, --context | string | Context to use in the configuration file. Defaults to the "default-context" value in the config file if set, or the first available context | Yes | Dependent |

## Configuration File

The CLI supports using a configuration file to keep track of commonly used parameters and flags.
The configuration file supports several **contexts** if you are working on several distinct Software Factory deployments
(for example, a dev instance and a production instance).

### Structure

```yaml
---
contexts:
  # the name of the context
  dev:
    # ALL FIELDS OPTIONAL
    # ---
    # the path to a local copy of the Software Factory's config repository
    config-repository-path: /path/to/config-repo
    # the path to the manifest defining the Software Factory deployment
    manifest-file: /path/to/manifest
    # specify whether the deployment was applied via an operator (standalone = false) or via the `apply` subcommand of the
    # CLI (standalone = true)
    standalone: false
    # the cluster's namespace on which to perform actions
    namespace: <namespace>
    # the kube context to use to perform actions, in case you have several contexts
    kube-context: <kube context>
    # the FQDN of the deployment if no manifest file is provided. Used mostly to interact with services' APIs.
    fqdn: <fqdn>
    # Developer settings
    development:
      # the path to a local copy of the ansible-microshift-role repository
      # (https://github.com/openstack-k8s-operators/ansible-microshift-role)
      ansible-microshift-role-path: /path/to/ansible-microshift-role/repository
      # Microshift deployment settings (used by the Ansible deployment playbook)
      microshift:
        host: microshift.dev
        user: cloud-user
        inventory-file: /path/to/inventory
      # Settings used when running the test suite locally
      tests:
        # Ansible extra variables to pass to the testing playbooks
        extra-vars:
          key1: value1
          key2: value2
    # SF components settings
    components:
      nodepool:
        # path to the local copy of the `clouds.yaml` file used by the Openstack provider
        clouds-file: /path/to/clouds-file
        # path to the local copy of the `kube.config` file used by the K8s-based providers
        kube-file: /path/to/kube-file
default-context: dev
```

## Subcommands

### Apply

The `apply` subcommand can be used to deploy a SoftwareFactory resource without installing the operator or its
associated CRDs on a cluster. This will run the operator runtime locally, deploy the resource's components
on the cluster, then exit.

```sh
go run ./main.go [GLOBAL FLAGS] apply --cr /path/to/manifest
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
|--cr |string | The path to the custom resource to apply | No | If a config file is used and the flag not provided, will default to the context's `manifest-file` if set |

### Operator

To start the operator controller locally, run:

```sh
go run ./main.go operator [FLAGS]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
|--metrics-bind-address |string  | The address the metric endpoint binds to. | Yes | :8080 |
|--health-probe-bind-address |string  | The address the probe endpoint binds to. | Yes | :8081 |
|--leader-elect |boolean  | Enable leader election for controller manager. | Yes | false |
### Backup

Not implemented yet

### Restore

Not implemented yet
