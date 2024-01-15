# CLI

The operator's `main.go` can be used to perform various actions related to the management of Software Factory
deployments, beyond what can be defined in a custom resource manifest.

## Table of Contents

1. [Running the CLI](#running-the-cli)
1. [Global Flags](#global-flags)
1. [Configuration File](#configuration-file)
1. [Subcommands](#subcommands)
  1. [Dev](#dev)
    1. [cloneAsAdmin](#cloneasadmin)
    1. [create gerrit](#create-gerrit)
    1. [create microshift](#create-microshift)
    1. [wipe gerrit](#wipe-gerrit)
  1. [Nodepool](#nodepool)
    1. [configure providers-secrets](#configure-providers-secrets)
    1. [get builder-ssh-key](#get-builder-ssh-key)
    1. [get providers-secrets](#get-providers-secrets)
  1. [Operator](#apply)
  1. [SF](#sf)
    1. [apply](#apply)
    1. [backup](#backup)
    1. [configure TLS](#configure-tls)
    1. [restore](#restore)
    1. [wipe](#wipe)

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
|-k, --kube-context |string | The cluster context on which to operate | Dependent |-|
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
      # path to a local copy of the sf-operator repository
      sf-operator-repository-path: /path/to/sf-operator/repository
      # Microshift deployment settings (used by the Ansible deployment playbook)
      microshift:
        host: microshift.dev
        user: cloud-user
        # The pull secret is required, see the MicroShift section of the developer documentation
        openshift-pull-secret: |
          PULL_SECRET
        # extra configuration settings
        # how much space to allocate for persistent volume storage
        disk-file-size: 30G
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

### Dev

The `dev` subcommand can be used to manage a development environment and run tests.

#### cloneAsAdmin

> ⚠️ A Gerrit instance must have been deployed with the [create gerrit](#create-gerrit) command first.

Clone a given repository hosted on the Gerrit instance, as the admin user. You can then proceed to
create patches and run them through CI with `git review`. If the repository already exists locally,
refresh it by resetting the remotes and performing a [hard reset](https://git-scm.com/docs/git-reset#Documentation/git-reset.txt---hard) on the master branch.

```sh
go run ./main.go [GLOBAL FLAGS] cloneAsAdmin REPO [DEST] [flags]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --verify | boolean | Enforce SSL validation | yes | False |

#### create gerrit

Create a Gerrit stateful set that can be used to host repositories and code reviews with a SF deployment.

```sh
go run ./main.go [GLOBAL FLAGS] dev create gerrit
```

To use this Gerrit instance with your Software Factory deployment, add the following to your SF manifest:

```yaml
[...]
spec:
  zuul:
    gerritconns:
      - name: gerrit
        username: zuul
        hostname: gerrit-sshd
        puburl: "https://gerrit.<FQDN>"
```

where `<FQDN>` is your domain name.

To host the config repository on this Gerrit instance, add the following to your SF manifest:

```yaml
[...]
spec:
  config-location:
    base-url: "http://gerrit-httpd/"
    name: config
    zuul-connection-name: gerrit
```

#### create microshift

Install and configure a MicroShift instance on a given server. This instance can then be used to host, develop and test the operator.

> ⚠️ `ansible-playbook` is required to run this command. Make sure it is installed on your system.

> ⚠️ the "Local Setup" step of the installation requires local root access to install required development dependencies. If you don't want to automate this process, run the command with the `--dry-run --skip-deploy --skip-post-install` flags to inspect the generated playbook and figure out what you need.

```sh
go run ./main.go [GLOBAL FLAGS] dev create microshift [FLAGS]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --dry-run | boolean | Create the playbooks but do not run them. | yes | False |
| --skip-local-setup | boolean | Do not install requirements and dependencies locally | yes | False |
| --skip-deploy | boolean | Do not install and start MicroShift on the target host | yes | False |
| --skip-post-install | boolean | Do not install operator dependencies, pre-configure namespaces | yes | False |

#### wipe gerrit

Delete a Gerrit instance deployed with `dev create gerrit`.

```sh
go run ./main.go [GLOBAL FLAGS] dev wipe gerrit [--rm-data]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --rm-data | boolean | Also delete persistent data (repositories, reviews) | yes | False |

### Nodepool

The `nodepool` subcommand can be used to interact with the Nodepool component of a Software Factory deployment.

#### configure providers-secrets

Set or update Nodepool's providers secrets (OpenStack's clouds.yaml and Kubernetes/OpenShift's kube.config).

> ⚠️ At least one of the `--kube` or `--clouds` flags must be provided.

```sh
go run ./main.go [GLOBAL FLAGS] nodepool configure providers-secrets [--kube /path/to/kube.config --clouds /path/to/clouds.yaml]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --kube | string | The file from which to read nodepool's kube.config | yes | - |
| --clouds | string | The file from which to read nodepool's clouds.yaml | yes | - |

#### get builder-ssh-key

The Nodepool builder component should be used with at least one `image-builder` companion machine.
It must have the capablility to connect via SSH to the builder machine(s). In order to do so, you need
to install the builder's SSH public key as an authorized key on the builder machine(s). This subcommand
fetches that key and can save it to a speficied file path.

```sh
go run ./main.go [GLOBAL FLAGS] nodepool get builder-ssh-key [--pubkey /path/to/key]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --pubkey | string | The destination file where to save the builder's public key | yes | - |

#### get providers-secrets

Get the currently set providers secrets (OpenStack's clouds.yaml and Kubernetes/OpenShift's kube.config) and optionally
write the secrets to a local file.

> ⚠️ The local files will be overwritten with the downloaded contents without warning!

```sh
go run ./main.go [GLOBAL FLAGS] nodepool get providers-secrets [--kube /path/to/kube.config --clouds /path/to/clouds.yaml]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --kube | string | The destination file where to save nodepool's kube.config | yes | - |
| --clouds | string | The destination file where to save nodepool's clouds.yaml | yes | - |

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

### SF

The following subcommands can be used to manage a Software Factory deployment and its lifecycle.

#### apply

The `apply` subcommand can be used to deploy a SoftwareFactory resource without installing the operator or its
associated CRDs on a cluster. This will run the operator runtime locally, deploy the resource's components
on the cluster, then exit.

```sh
go run ./main.go [GLOBAL FLAGS] SF apply --cr /path/to/manifest
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
|--cr |string | The path to the custom resource to apply | No | If a config file is used and the flag not provided, will default to the context's `manifest-file` if set |

#### backup

Not implemented yet

#### configure TLS

The `configure TLS` subcommand can be used to inject a pre-existing set of certificates to secure
the HTTPS endpoints of a Software Factory deployment.

```sh
go run ./main.go [GLOBAL FLAGS] SF configure TLS --CA /path/to/CA --cert /path/to/cert --key /path/to/privatekey
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --CA | string | The path to the PEM-encoded Certificate Authority file | no | - |
| --cert | string | The path to the domain certificate file | no | - |
| --key | string | The path to the private key file | no | - |

#### restore

Not implemented yet

#### wipe

The `wipe` subcommand can be used to remove all Software Factory instances in the provided namespace,
their persistent volumes, and even remove the SF operator completely.

The default behavior is to stop and remove all containers related to a Software Factory deployment, and
keep the existing persistent volumes.

```sh
go run ./main.go [GLOBAL FLAGS] SF wipe [FLAGS]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --rm-data | boolean | Also delete all persistent volumes after removing the instances | yes | False |
| --all | boolean | Remove all data like with the `--rm-data` flag, and remove the operator from the cluster | yes | False |