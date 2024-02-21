.# Zuul

Here you will find information about managing the Zuul service when deployed with the SF Operator.
It does not replace [Zuul's documentation](https://zuul-ci.org/docs/zuul/latest/),
but addresses specificities and idiosyncrasies induced by deploying Zuul with the SF Operator.


1. [Architecture](#architecture)
1. [Services configuration](#services-configuration)
1. [Tenant configuration](#tenant-configuration)
1. [Delegating temporary administrative powers on a tenant](#delegating-temporary-administrative-powers-on-a-tenant)
1. [Zuul-Client](#zuul-client)
1. [Zuul-Admin](#zuul-admin)
1. [Scaling Zuul](#scaling-zuul)

## Architecture

Zuul is deployed by SF-Operator as micro-services:

| Service | Kubernetes resource type | Scalable Y/N |
|---------|--------------------------|-------------|
| zuul-scheduler | statefulset | N |
| zuul-executor | statefulset | Y |
| zuul-web | deployment | N |

## Services configuration

Configuring the Zuul micro-services is done through the SoftwareFactory deployment's manifest. Many configuration parameters are exposed by The [SoftwareFactory Custom Resource spec](../deployment/crds.md#softwarefactory).

The spec is constantly evolving during alpha development, and should be considered
unstable but the ultimate source of truth for documentation about its properties.

## Tenant configuration

Zuul's tenant configuration is stored in the [config repository](./config_repository.md). Edit `./zuul/main.yaml` to add, edit or delete tenants and projects on your
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

The `zuul-admin` command line utility is available on `zuul-scheduler` pods.

Open a shell on any available `zuul-scheduler` pod, for example `zuul-scheduler-0`:

```sh
kubectl exec --stdin --tty zuul-scheduler-0 -- /bin/sh
```

Then from that shell, run the `zuul-admin` command.

## Scaling Zuul

Zuul Executor and Zuul Merger services can be scaled whenever sf-operator deployment
no more fits the demand of the CI jobs.
The scale is done with Kubernetes scale cli command:
```bash
kubectl scale <resource kind> <resource name> --replicas=<number of replicas>

# Example to scale Zuul Executor
kubectl scale sts zuul-executor --replicas=3
```
The scale will take no more that one minute.
