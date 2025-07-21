# Zuul

Here you will find information about managing the Zuul service when deployed with the SF Operator.
It does not replace [Zuul's documentation](https://zuul-ci.org/docs/zuul/latest/),
but addresses specificities and idiosyncrasies of deploying Zuul with the SF Operator.


1. [Architecture](#architecture)
1. [Services configuration](#services-configuration)
1. [Tenant configuration](#tenant-configuration)
1. [Delegating temporary administrative powers on a tenant](#delegating-temporary-administrative-powers-on-a-tenant)
1. [Zuul-Client](#zuul-client)
1. [Zuul-Admin](#zuul-admin)
1. [Scaling Zuul](#scaling-zuul)

## Architecture

Zuul is deployed by SF-Operator as micro-services:

| Component | Kubernetes resource type | Scalable Y/N |
|---------|--------------------------|-------------|
| zuul-scheduler | statefulset | N |
| zuul-executor | statefulset | Y |
| zuul-merger | statefulset | Y |
| zuul-web | deployment | N |


### Zuul-scheduler Pod

| Container | Type | R.Request (Mem/CPU) | R.Limit (Mem/CPU) | R.Limit Upgrade (Y/N) |
|---------|--------------------------|-------------|-----| --- |
| zuul-scheduler | normal | 128Mi/0.1CPU | 2Gi/2CPU | Y |
| zuul-scheduler-nodeexporter | normal | 32Mi/0.01CPU | 64Mi/0.1CPU | N |
| zuul-statsd | normal | 32Mi/0.01CPU | 64Mi/0.1CPU | N |
| init-scheduler-config | init | 32Mi/0.01CPU | 64Mi/0.1CPU | N |

### Zuul-executor Pod

| Container | Type | R.Request (Mem/CPU) | R.Limit (Mem/CPU) | R.Limit Upgrade (Y/N) |
|---------|--------------------------|-------------|-----| --- |
| zuul-executor | privileged | 128Mi/0.1CPU | 2Gi/2CPU | Y |
| zuul-executor-nodeexporter | normal | 32Mi/0.01CPU | 64Mi/0.1CPU | N |

### Zuul-merger Pod

| Container | Type | R.Request (Mem/CPU) | R.Limit (Mem/CPU) | R.Limit Upgrade (Y/N) |
|---------|--------------------------|-------------|-----| --- |
| zuul-merger | normal | 128Mi/0.1CPU | 2Gi/2CPU | Y |
| zuul-merger-nodeexporter | normal | 32Mi/0.01CPU | 64Mi/0.1CPU | N |

### Zuul-web Pod

| Container | Type | R.Request (Mem/CPU) | R.Limit (Mem/CPU) | R.Limit Upgrade (Y/N) |
|---------|--------------------------|-------------|-----| --- |
| zuul-web | normal | 128Mi/0.1CPU | 2Gi/2CPU | Y |

## Resource limits

For each component, the main container's resource limits can be changed via the `SoftwareFactory` Custom Resource.

## Services configuration

Configuring the Zuul micro-services is done through the SoftwareFactory deployment's manifest. Many configuration parameters are exposed by the [SoftwareFactory Custom Resource spec](../deployment/crds.md#softwarefactory).

The spec is constantly evolving during alpha development and should be considered unstable, but it is the ultimate source of truth for documentation about its properties.

## Tenant configuration

Zuul's tenant configuration is stored in the [config repository](./config_repository.md). Edit `./zuul/main.yaml` to add, edit, or delete tenants and projects on your
deployment; then commit your changes for review and CI validation.

## Delegating temporary administrative powers on a tenant

Zuul can generate temporary tokens to use with `zuul-client`. These tokens allow
a user to perform administrative tasks such as managing autoholds, promoting changes
 and re-enqueueing buildsets on a given tenant. This feature is documented [here](https://zuul-ci.org/docs/zuul/latest/client.html#create-auth-token) in
 Zuul's upstream documentation.

There are two ways to generate such a token:

=== "sf-operator"

    ```bash
    sf-operator zuul create auth-token [FLAGS]
    ```

    Refer to [the command's documentation](../reference/cli/index.md#create-auth-token) for further details.

=== "kubectl exec"

    ```sh
    kubectl exec -n sf --stdin --tty zuul-scheduler-0 -- zuul-admin create-auth-token [...]
    ```

## Zuul-Client

You can [generate a configuration file for zuul-client](../reference/cli/index.md#create-client-config) with the `sf-operator` CLI.

## Zuul-Admin

The `zuul-admin` command-line utility is available on `zuul-scheduler` pods.

Open a shell on any available `zuul-scheduler` pod, for example `zuul-scheduler-0`:

```sh
kubectl exec --stdin --tty zuul-scheduler-0 -- /bin/sh
```

Then from that shell, run the `zuul-admin` command.

## Scaling Zuul

Zuul Executor and Zuul Merger services can be scaled whenever the sf-operator deployment
no longer fits the demand of the CI jobs.
The scaling is done with the Kubernetes scale CLI command:
```bash
kubectl scale <resource kind> <resource name> --replicas=<number of replicas>

# Example to scale Zuul Executor
kubectl scale sts zuul-executor --replicas=3
```
The scaling will take no more than one minute.
