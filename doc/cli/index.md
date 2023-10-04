# sfconfig

SF-Operator comes with a Command Line Interface (CLI) called `sfconfig` that can be used to perform actions in relation with the lifecycle
of the operator, of a deployment, or also to help with the development of the operator.

## Table of Contents

1. [Running sfconfig](#running-sfconfig)
1. [sfconfig.yaml](#sfconfigyaml)
1. [Operator management commands](#operator-management-commands)
1. [Deployment management commands](#deployment-management-commands)
1. [Deployment user commands](#deployment-user-commands)
1. [Development-related commands](#development-related-commands)

## Running sfconfig

The CLI can be run with

```sh
./tools/sfconfig
```

And contextual help is available with 

```sh
./tools/sfconfig [COMMAND] --help
```

Many options of the CLI can be passed through the `sfconfig.yaml` file. The path to this file can be passed to the CLI as the `--config` argument.

## sfconfig.yaml

The `sfconfig.yaml` file is used to pass some common parameters to the CLI. Here is its default structure:

```yaml
ansible_microshift_role_path: ~/src/github.com/openstack-k8s-operators/ansible-microshift-role
microshift:
  host: microshift.dev
  user: cloud-user
fqdn: sfop.dev
nodepool:
  clouds_file: /etc/sf-operator/nodepool/clouds.yaml
  kube_file: /etc/sf-operator/nodepool/kubeconfig.yaml
```

## Operator management commands

### operator

#### create

This command can be used to prepare namespaces on your cluster for the SF-Operator, for a SoftwareFactory deployment, and a namespace dedicated to running pods with Nodepool.

Usage:
```sh
sfconfig operator create [flags]
```
Flags:

| Argument | Type | Description | Default |
|----------|------|-------|----|
|-a, --all    | boolean |                executes all options in sequence|-|
|-b, --bundle    |   boolean |           creates namespace for the bundle | operators|
|-B, --bundlenamespace| string |  creates namespace for the bundle with specific name|-|
|-n, --namespace   |   boolean |         creates namespace for Software Factory |sf|
|-N, --namespacename |string   |  creates namespace for Software Factory with specific name|-|
|-v, --verbose     |   boolean |         verbose|-|

#### delete


This command can be used to wipe SF-Operator off a cluster.

Usage:
```sh
sfconfig operator delete [flags]
```

Flags:
| Argument | Type | Description | Default |
|----------|------|-------|----|
|  -a, --all   |      boolean           | executes all options in sequence|-|
|  -S, --catalogsource  |   boolean     | deletes Software Factory Catalog Source|-|
|  -c, --clusterserviceversion| boolean | deletes Software Factory Cluster Service Version|-|
|  -s, --subscription  |    boolean     | deletes Software Factory Operator's Subscription|-|


## Deployment management commands

### General
#### create-service-ssl-secret

This command configures a secret with the provided SSL context, and updates the given service's HTTPS route to use this context.

Usage:

```sh
sfconfig create-service-ssl-secret [flags]
```

Flags:

| Argument | Type | Description | Default |
|----------|------|-------|----|
|--sf-context |string    |    The kubeconfig context of the sf-namespace,| Default context|
|--sf-namespace |string   |   Name of the namespace to copy the kubeconfig, or '-' for stdout |sf|
|--sf-service-ca |string   |  Path for the service CA certificate| - |
|--sf-service-cert |string  | Path for the service certificate file| - |
|--sf-service-key |string   | Path for the service private key file| - |
|--sf-service-name s|tring  | The SF service name for the SSL certificate like Zuul, Gerrit, Logserver etc.| - |

See [this section in the deployment documentation](./../deployment/certificates.md#using-x509-certificates) for more details.

#### sf delete

This command can be used to wipe a deployment at the desired depth.

Usage:
```sh
sfconfig sf delete [flags]
```

Flags:
| Argument | Type | Description | Default |
|----------|------|-------|----|
|  -a, --all  |  boolean  |  executes --delete and --remove options in sequence|-|
|  -i, --instance|boolean|  deletes Software Factory Instance|-|
|  -p, --pvcs |  boolean  |  deletes Software Factory including PVCs and PVs|-|
|  -v, --verbose |boolean |  verbose|-|


### Nodepool
#### create-namespace-for-nodepool

This command creates 

* a suitable namespace on the OpenShift cluster,
* a kube configuration set to this namespace,

that can be used with Nodepool's `openshiftpods` driver. if the `--sf-namespace` argument is set to
your deployment's namespace, `sfconfig` will also update Nodepool's secrets on your deployment.

Usage:

```sh
sfconfig create-namespace-for-nodepool [flags]
```

Flags:

| Argument | Type | Description | Default |
|----------|------|-------|----|
| --nodepool-context | string  |   The kubeconfig context for the nodepool-namespace| Default context |
|  --nodepool-namespace | string |  The namespace name for nodepool |nodepool|
|  --sf-context | string          | The kubeconfig context of the sf-namespace| Default context|
|  --sf-namespace | string     |    Name of the namespace to copy the kubeconfig, or '-' for stdout |sf |

See [this section in the deployment documentation](./../deployment/nodepool.md#using-the-openshiftpods-driver-with-your-cluster) for more details.

#### nodepool-providers-secrets

This command provides capabilities to dump and update a clouds.yaml and a kube.config file for Nodepool

Usage:
```sh
sfconfig nodepool-providers-secrets [flags]
```

Flags:
| Argument | Type | Description | Default |
|----------|------|-------|----|
|  -d, --dump  |       boolean     |    Dump the providers secrets from the sf namespace to the local config (exclusive with '-u')|-|
|     --sf-context| string  |   The kubeconfig context of the sf-namespace| Default context|
|     --sf-namespace| string |  Name of the namespace to copy the kubeconfig, or '-' for stdout |sf|
| -u, --update  |       boolean     |  Update the providers secrets from local config to the sf namespace (exclusive with '-d')|-|

See [this section in the deployment documentation](./../deployment/nodepool.md#setting-up-provider-secrets) for more details.

### Zuul

#### create-auth-token

This command is a proxy for the "zuul-admin create-auth-token" command run on a scheduler pod.

The command will output a JWT that can be passed to the zuul-client CLI or used with cURL to perform
administrative actions on a specified tenant.

Usage:
```sh
sfconfig zuul create-auth-token [flags]
```

Flags:
| Argument | Type | Description | Default |
|----------|------|-------|----|
|  -x, --expires-in| int32 |  The lifespan in seconds of the token. | 15 minutes (900s)|
|  -n, --namespace| string|   Name of the namespace where Zuul is deployed| "sf"|
|  -t, --tenant| string  |    The Zuul tenant on which to grant administrative powers| "local"|
|  -u, --user| string   |     A username for the token holder. Used for logs auditing only |"cli_user"|

See [Zuul's upstream documentation](https://zuul-ci.org/docs/zuul/latest/client.html#create-auth-token)
for more details.

#### zuul-client

This command provides a "proxy" of sorts to the `zuul-client` CLI.

Usage:
```sh
sfconfig zuul-client [...]
```

See the [zuul-client documentation](https://zuul-ci.org/docs/zuul-client/) for more details.

## Deployment user commands

### bootstrap-tenant-config-repo

This command creates a scaffolding in a repository that can be modified to enable Zuul to trigger jobs on various git events.

Usage:
```sh
sfconfig bootstrap-tenant-config-repo [...]
```

Flags:
| Argument | Type | Description | Default |
|----------|------|-------|----|
|      --connection |string  | Name of the connection or a source|-|
|      --outpath |string    | Path to create file structure|-|

See the [user documemtation](./../user/index.md) for more details.

## Development-related commands

### gerrit
This command manages the lifecycle of a demo Gerrit instance to hack on sf-operator.

Usage:
```sh
sfconfig gerrit [flags]
```

Flags:
| Argument | Type | Description | Default |
|----------|------|-------|----|
|      --deploy |  boolean   |  Deploy Gerrit on the cluster|-|
|  -f, --fqdn| string |  The FQDN of gerrit (gerrit.FQDN)|sfop.dev|
|      --wipe |   boolean    |  Wipe Gerrit deployment|-|

### microshift

This command can be used to set up a test MicroShift cluster, similar to the ones that are used
for this project's CI and development.

Usage:
  sfconfig microshift [flags]

Flags:
| Argument | Type | Description | Default |
|----------|------|-------|----|
|  -i, --inventory |string  | Specify ansible playbook inventory|-|
|      --skip-deploy |   boolean   | do not deploy microshift|-|
|      --skip-local-setup| boolean  |do not install local requirements|-|

See [this section in the developer documentation](./../developer/microshift.md) for more details.
### prometheus

This command manages the lifecycle of a demo Prometheus instance to experiment with operand monitoring.

Usage:
```sh
sfconfig prometheus [flags]
```

Flags:
| Argument | Type | Description | Default |
|----------|------|-------|----|
|  -f, --fqdn| string |  The FQDN for prometheus (prometheus.FQDN)|sfop.dev|
|      --skip-operator-installation |   boolean    | Do not attempt to install the prometheus operator prior to deploying a Prometheus instance|-|

### runTests

This command enables running the CI test suite locally. This is basically a wrapper to the project's Ansible test playbooks.

Usage:
```sh
sfconfig runTests [flags]
```

Flags:
| Argument | Type | Description | Default |
|----------|------|-------|----|
|  -e, --extra-var |string |  Set extra variables, the format of each variable must be `key`=`value`|-|
|  -h, --help      | boolean       |  help for runTests|-|
|  -t, --test-only |   boolean     |  run tests only - it is assumed the operator is running and a SoftwareFactory resource is already deployed |-|
|  -u, --upgrade  |   boolean      |  run upgrade test|-|
|      --v        |boolean         |  run ansible in verbose mode|-|
|      --vvv      |  boolean      |   run ansible in debug mode|-|
