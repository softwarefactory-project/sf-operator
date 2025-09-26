# Nodepool

Here you will find information about managing the Nodepool service when deployed with the SF Operator.
It does not replace [Nodepool's documentation](https://zuul-ci.org/docs/nodepool/latest/),
but addresses specificities and idiosyncrasies of deploying Nodepool with the SF Operator.


1. [Architecture](#architecture)
1. [Services configuration](#services-configuration)
1. [Setting up provider secrets](#setting-up-providers-secrets)
1. [Get the builder's SSH public key](#get-the-builders-ssh-public-key)
1. [Using the openshiftpods driver with your cluster](#using-the-openshiftpods-driver-with-your-cluster)
1. [Using the Nodepool CLI](#using-the-nodepool-cli)
1. [Troubleshooting](#troubleshooting)

## Architecture

Nodepool is deployed by SF-Operator as micro-services:

| Service | Kubernetes resource type | Scalable Y/N |
|---------|--------------------------|-------------|
| nodepool-launcher | deployment | N |
| nodepool-builder | statefulset | N |

`nodepool-builder` requires access to at least one `image-builder` machine that is to be deployed out-of-band. Due to security limitations,
it is impossible (or at least very hard) to build Nodepool images in a pod, which is why this process must be delegated remotely to an `image-builder` machine.

!!! note
    The only requirement for the `image-builder` machine is that the `nodepool-builder` is able to run Ansible tasks via SSH. Please refer to sections [Get the builder's SSH public key](#get-the-builders-ssh-public-key) and [Configuring Nodepool builder](../user/nodepool_config_repository.md#configuring-nodepool-builder).

!!! note
    There is no assumption about the processes and tooling used to build images on the `image-builder`, except that the workflow must be driven by an Ansible playbook from the `nodepool-builder`.

## Services configuration

Configuring the Nodepool micro-services is done through the SoftwareFactory deployment's manifest. Many configuration parameters are exposed by the [SoftwareFactory Custom Resource spec](../deployment/crds.md#softwarefactory).

The spec is constantly evolving during alpha development and should be considered unstable, but it is the ultimate source of truth for documentation about its properties.

## Setting up providers secrets

Currently the SF Operator supports OpenStack (`clouds.yaml`) and Kubernetes (`kube.config`) configuration files. These files are used by Nodepool to manage resources on its providers.
They are managed by the SF Operator in a secret called `nodepool-providers-secrets`.

```sh
kubectl create secret generic nodepool-providers-secrets --from-file=clouds.yaml=<path-to>/clouds.yaml --from-file=kube.config=<path-to>/kube.config --dry-run=client -o yaml | kubectl apply -f -
```

Then wait until your deployment becomes ready again:

```sh
kubectl get sf -n sf -w
```

When your deployment is ready, the provider secrets have been updated in Nodepool.

## Get the builder's SSH public key

The Nodepool builder component should be used with at least one `image-builder` companion machine.
It must have the capability to connect via SSH to the builder machine.

There are two ways to fetch the builder SSH public key: with kubectl, or with the [sf-operator CLI](../reference/cli/index.md).

=== "sf-operator"

    ```sh
    sf-operator --namespace sf nodepool get builder-ssh-key
    ```

=== "kubectl"

    ```sh
    kubectl get secret nodepool-builder-ssh-key -n sf -o jsonpath={.data.pub} | base64 -d
    ```

## Accept an image-builder's SSH Host key

Once an account has been created on an `image-builder` host, the `nodepool-builder` must trust the SSH host key before being able to connect. Run the following command to initiate a SSH connection and trust the host key:

```sh
kubectl exec -it nodepool-builder-0 -c nodepool-builder -- ssh nodepool@<image-builder-hostname> hostname
```

## Using the Openshiftpods driver with your cluster

Nodepool's [Openshiftpods driver](https://zuul-ci.org/docs/nodepool/latest/openshift-pods.html) enables
Zuul to request pods from an OpenShift cluster to run jobs.

We recommend using a dedicated namespace with the driver (if you intend to spawn pods in the same OpenShift cluster as the one your deployment lives in).

The [`sf-operator` CLI](./../reference/cli/index.md#create-openshiftpods-namespace) can automate the creation of such a namespace, and set up the required kube config as a Nodepool secret:

```sh
sf-operator [GLOBAL FLAGS] nodepool create openshiftpods-namespace [FLAGS]
```

Once Nodepool is ready, add the following snippet in the `nodepool/nodepool.yaml` file in your `config` repository:

```yaml
providers:
  - name: openshift-pods
    driver: openshiftpods
    context: my-context
    pools:
      - name: main
        labels:
          - name: my-pod
            image: quay.io/fedora/fedora:latest
```

Commit your change, review it and validate it. After a run of `config-update`, your new provider and
labels will be available in Nodepool.

## Using the Nodepool CLI

The `nodepool` command-line utility is available on `nodepool-launcher` pods.

To get the list of currently running launcher pods:

```sh
kubectl get pods --selector run=nodepool-launcher
```

Open a shell on any running `nodepool-launcher` pod listed by the previous command:

```sh
kubectl exec --stdin --tty nodepool-launcher-XYZ -- /bin/sh
```

Then from that shell, run the `nodepool` command.

## Troubleshooting

### How to connect to a ready node from the launcher pod

`nodepool list` is used to list all nodes managed by the launcher and their current status.

```sh
$ kubectl exec -ti nodepool-launcher-$uuid -c launcher -- nodepool list
```

Look for the node's IP address then from the Zuul executor pod, run:

```sh
$ kubectl exec -ti zuul-executor-0 -- ssh -o "StrictHostKeyChecking no" <user>@<ip>
Warning: Permanently added '$public_ip' (ED25519) to the list of known hosts.
$ hostname
np0000000001
```

### Accessing the Nodepool API

Nodepool exposes some [API endpoints](https://zuul-ci.org/docs/nodepool/latest/operation.html#web-interface).

For instance, to reach the `image-list` endpoint, a user can access the following URL: `https://<fqdn>/nodepool/api/image-list`.
