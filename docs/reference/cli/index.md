# CLI

We provide a command to perform various actions related to the management of Software Factory
deployments, beyond what can be defined in a custom resource manifest.


- [Installing the CLI](#installing-the-cli)
- [Global Flags](#global-flags)
- [Configuration File](#configuration-file)
- [Subcommands](#subcommands)
  - [Dev](#dev)
    - [cloneAsAdmin](#cloneasadmin)
    - [create demo-env](#create-demo-env)
    - [create gerrit](#create-gerrit)
    - [create microshift](#create-microshift)
    - [getImagesSecurityIssues](#getimagessecurityissues)
    - [run-tests](#run-tests)
    - [wipe gerrit](#wipe-gerrit)
    - [wipe sf](#wipe-sf)
  - [Init](#init)
  - [Nodepool](#nodepool)
    - [create openshiftpods-namespace](#create-openshiftpods-namespace)
    - [get builder-ssh-key](#get-builder-ssh-key)
  1. [Operator](#operator)
  1. [SF](#sf)
    1. [backup](#backup)
    1. [bootstrap-tenant](#bootstrap-tenant)
    1. [configure TLS](#configure-tls)
    1. [restore](#restore)
  1. [Zuul](#zuul)
    - [create auth-token](#create-auth-token)
    - [create client-config](#create-client-config)
  1. [Deploy](#deploy)
  1. [Version](#version)

## Installing the CLI

To build the CLI, assuming you are at the root directory of the sf-operator repository:

```sh
go install
```

Then the CLI can be used with:

```sh
sf-operator [GLOBAL FLAGS] [ARGS] [SUBCOMMAND FLAGS]...
```

## Global Flags

These flags apply to every subcommand.

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
        host: microshift.sfci
        user: cloud-user
        # The pull secret is required, see the MicroShift section of the developer documentation
        openshift-pull-secret: |
          PULL_SECRET
        # extra configuration settings
        # how much space to allocate for persistent volume storage
        disk-file-size: 30G
      # Settings used when running the test suite locally
      tests:
        # where to check out/create the demo repositories used by tests
        demo-repos-path: deploy/
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

!!! warning
    A Gerrit instance must have been deployed with the [create gerrit](#create-gerrit) command first.

Clone a given repository hosted on the Gerrit instance, as the admin user. You can then proceed to
create patches and run them through CI with `git review`. If the repository already exists locally,
refresh it by resetting the remotes and performing a [hard reset](https://git-scm.com/docs/git-reset#Documentation/git-reset.txt---hard) on the master branch.

```sh
sf-operator [GLOBAL FLAGS] cloneAsAdmin REPO [DEST] [flags]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --verify | boolean | Enforce SSL validation | yes | False |

#### create demo-env

Create a Gerrit instance if needed, then clone and populate demo repositories that will set up
a demo tenant.

!!! warning
    This command will also install the operator's Custom Resource Definitions, so you need to run `make manifests` beforehand.

```sh
sf-operator [GLOBAL FLAGS] dev create demo-env [FLAGS]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --keep-tenant-config | boolean | Do not update the demo tenant configuration | yes | False |
| --repos-path | string | Where to clone the demo repositories | yes | ./deploy/ |

#### create gerrit

Create a Gerrit stateful set that can be used to host repositories and code reviews with a SF deployment.

```sh
sf-operator [GLOBAL FLAGS] dev create gerrit
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
    name: config
    zuul-connection-name: gerrit
```

#### create microshift

Install and configure a MicroShift instance on a given server. This instance can then be used to host, develop and test the operator.

!!! warning
    `ansible-playbook` is required to run this command. Make sure it is installed on your system.

```sh
sf-operator [GLOBAL FLAGS] dev create microshift [FLAGS]
```

!!! warning
    The "Local Setup" step of the installation requires local root access to install required development dependencies. If you don't want to automate this process, run the command with the `--dry-run --skip-deploy --skip-post-install` flags to inspect the generated playbook and figure out what you need.

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --dry-run | boolean | Create the playbooks but do not run them. | yes | False |
| --skip-local-setup | boolean | Do not install requirements and dependencies locally | yes | False |
| --skip-deploy | boolean | Do not install and start MicroShift on the target host | yes | False |
| --skip-post-install | boolean | Do not install operator dependencies, pre-configure namespaces | yes | False |

#### run-tests

Run the playbook for a given test suite. Extra variables can be specified.

```sh
sf-operator [GLOBAL FLAGS] dev run-tests {olm,standalone,upgrade} [FLAGS]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
|--extra-var | string | Set an extra variable in the form `key=value` to pass to the test playbook. Repeatable | Yes | - |
|--v | boolean | Run playbook in verbose mode | Yes | false |
|--vvv | boolean | Run playbook in debug mode | Yes | false |
|--prepare-demo-env | boolean | Prepare a demo environment before running the test suite (see [dev create demo-env](#create-demo-env)) | Yes | false |

#### wipe gerrit

Delete a Gerrit instance deployed with `dev create gerrit`.

```sh
sf-operator [GLOBAL FLAGS] dev wipe gerrit [--rm-data]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --rm-data | boolean | Also delete persistent data (repositories, reviews) | yes | False |

#### getImagesSecurityIssues

To get a report of security issues reported by quay.io for container images used by the
sf-operator, run: `dev getImagesSecurityIssues`. This command helps to decide if we need to
rebuild container images to benefit from the latest security fixes from the base OS.

```sh
sf-operator dev getImagesSecurityIssues
```

#### wipe sf

Delete a deployed Software Factory and optionally delete the data (PVC) and the
sf-operator deployment.

```sh
sf-operator [GLOBAL FLAGS] dev wipe sf [FLAGS]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --rm-data | boolean | Delete also persistent data | yes | False |
| --rm-operator | boolean | [sf] Delete also the operator installation | yes | False |

### Init

The `init` subcommand can be used to initialize a CLI configuration file, or a sample manifest for deploying Software Factory.

#### config

Generate a simple CLI configuration tree with one context. It is up to you to save it to a chosen
file, and to edit it to suit your requirements.

```sh
sf-operator [GLOBAL FLAGS] init config [--dev] > /path/to/sfcli.config
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --dev | boolean | Include development-related configuration parameters | yes | False |

#### manifest

Generate a basic Software Factory manifest that can be used to deploy a Software Factory. It is up to you to
save it a chosen file, edit it as you see fit and then apply it to your cluster.

```sh
sf-operator [GLOBAL FLAGS] init manifest [FLAGS] > /path/to/sf.yaml
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --connection | string, repeatable | Include connection. The first connection of the list will be assumed to host the deployment's config repository | yes | gerrit |
| --full | boolean | Include optional fields in the manifest, for a more fine-tuned deployment | yes | False |
| --with-auth | boolean | Include OIDC authentication configuration | yes | False |
| --with-builder | boolean | Include nodepool builder configuration | yes | False |

### Nodepool

The `nodepool` subcommand can be used to interact with the Nodepool component of a Software Factory deployment.

#### create openshiftpods-namespace

Create and set up a dedicated namespace on a cluster, so that nodepool can spawn pods with the [openshiftpods](https://zuul-ci.org/docs/nodepool/latest/openshift-pods.html) driver.

!!! note
    See [this section in the deployment documentation](../../deployment/nodepool.md#using-the-openshiftpods-driver-with-your-cluster) for more details.

```sh
sf-operator [GLOBAL FLAGS] nodepool create openshiftpods-namespace [FLAGS]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --nodepool-context | string | The kube context to use to set up the namespace | yes | default context set with `kubectl` |
| --nodepool-namespace | string | The namespace to set up | yes | nodepool |
| --show-config-template | boolean | Display a nodepool configuration snippet that can be used to enable an openshiftpods provider using the created namespace | yes | false |
| --skip-providers-secrets | boolean | Do not update or create nodepool's providers secrets after setting up the namespace | yes | false |

#### get builder-ssh-key

The Nodepool builder component should be used with at least one `image-builder` companion machine.
It must have the capability to connect via SSH to the builder machine(s). In order to do so, you need
to install the builder's SSH public key as an authorized key on the builder machine(s). This subcommand
fetches that key and can save it to a specified file path.

```sh
sf-operator [GLOBAL FLAGS] nodepool get builder-ssh-key [--pubkey /path/to/key]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --pubkey | string | The destination file where to save the builder's public key | yes | - |

### Operator

To start the operator controller locally, run:

```sh
sf-operator operator [FLAGS]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
|--metrics-bind-address |string  | The address the metric endpoint binds to. | Yes | :8080 |
|--health-probe-bind-address |string  | The address the probe endpoint binds to. | Yes | :8081 |
|--leader-elect |boolean  | Enable leader election for controller manager. | Yes | false |

### SF

The following subcommands can be used to manage a Software Factory deployment and its lifecycle.

#### backup

The `backup` subcommand lets you dump a Software Factory's most important files for safekeeping.

To create a backup located in `/tmp/backup` directory, run the following command:

```sh
sf-operator SF backup --namespace sf --backup_dir /tmp/backup
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --backup_dir | string | The path to the backup directory. | no | - |

The backup is composed of the following:

- some relevant `Secrets` located in the deployment's namespace
- the Zuul's SQL database
- the Zuul's project's keys as exported by [zuul-admin export-keys](https://zuul-ci.org/docs/zuul/latest/client.html#export-keys)

The backup directory content can be compressed and stored safely in a backup system.

#### bootstrap-tenant

Initialize a Zuul tenant's config repository with boilerplate code that defines standard pipelines:

* "check" for pre-commit validation
* "gate" for approved commits gating
* "post for post-commit actions

It also includes a boilerplate job and pre-run playbook.

```sh
sf-operator SF bootstrap-tenant /path/to/tenant-config-repo [FLAGS]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
|--connection |string  | The name of the Zuul connection to use for pipelines | No | - |
|--driver |string  | The driver used by the Zuul connection. Supported drivers: gerrit, gitlab | No | - |

#### configure TLS

The `configure TLS` subcommand can be used to inject a pre-existing set of certificates to secure
the HTTPS endpoints of a Software Factory deployment.

```sh
sf-operator [GLOBAL FLAGS] SF configure TLS --CA /path/to/CA --cert /path/to/cert --key /path/to/privatekey
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --CA | string | The path to the PEM-encoded Certificate Authority file | no | - |
| --cert | string | The path to the domain certificate file | no | - |
| --key | string | The path to the private key file | no | - |

#### restore

!!! warning
    The command requires you to have the `kubectl` binary installed on the system

The `restore` subcommand lets you restore a backup created with the `backup` command.

For example:

```sh
sf-operator SF restore --namespace sf --backup_dir my_backup_dir
```

Available flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --backup_dir | string | The path to the backup directory to restore | yes | - |

### Zuul

These subcommands can be used to interact with the Zuul component of a deployment.

#### create auth-token

The `create auth-token` subcommand can be used to create a custom authentication token that can be used with the [zuul-client CLI utility](https://zuul-ci.org/docs/zuul-client/).

```sh
sf-operator [GLOBAL FLAGS] zuul create auth-token [FLAGS]
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --auth-config | string | The authentication configuration to use | yes | zuul_client |
| --tenant | string | The tenant on which to grant admin access | no | - |
| --user | string | a username, used for audit purposes in Zuul's access logs | yes | "John Doe" |
| --expiry | int | How long in seconds the authentication token should be valid for | yes | 3600 |

#### create client-config

The `create client-config` generates a configuration file that can be used with the [zuul-client CLI utility](https://zuul-ci.org/docs/zuul-client/) against the Software Factory deployment.

!!! warning
    The command provisions authentication tokens that grant admin access to all tenants. It is recommended to review and eventually edit the output of the command before forwarding it to third parties.

```sh
sf-operator [GLOBAL FLAGS] zuul create client-config [FLAGS] > zuul-client.conf
```

Flags:

| Argument | Type | Description | Optional | Default |
|----------|------|-------|----|----|
| --auth-config | string | The authentication configuration to use | yes | zuul_client |
| --tenant | string | The tenant on which to grant admin access | no | - |
| --user | string | a username, used for audit purposes in Zuul's access logs | yes | "John Doe" |
| --expiry | int | How long in seconds the authentication token should be valid for | yes | 3600 |
| --insecure | boolean | skip SSL validation when connecting to Zuul | yes | False |

### Deploy

Deploy a "standalone" Software Factory. In standalone mode, you do not need to install or run the operator
controller within your cluster. You do not need to install the Software Factory CRDs as well. The CLI will take
a Software Factory manifest as input and deploy all the required services as a one-shot.

```sh
sf-operator [GLOBAL FLAGS] deploy /path/to/manifest
```

### Version

Return the version of the executable. If run directly without building the executable first (i.e. with `go run ./main.go`),
this command requires `git` to be installed in the environment. It must also be running from the project's source directory.