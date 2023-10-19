# Nodepool

Here you will find information about managing the Nodepool service when deployed with the SF Operator.
It does not replace [Nodepool's documentation](https://zuul-ci.org/docs/nodepool/latest/),
but addresses specificities and idiosyncrasies induced by deploying Nodepool with the SF Operator.

## Table of Contents

1. [Architecture](#architecture)
1. [Services configuration](#services-configuration)
1. [Setting up providers secrets](#setting-up-providers-secrets)
1. [Get the builder's SSH public key](#get-the-builders-ssh-public-key)
1. [Using the openshifpods driver with your cluster](#using-the-openshiftpods-driver-with-your-cluster)
1. [Using the Nodepool CLI](#using-the-nodepool-cli)
1. [Troubleshooting](#troubleshooting)

## Architecture

Nodepool is deployed by SF-Operator as micro-services:

| Service | Kubernetes resource type | Scalable Y/N |
|---------|--------------------------|-------------|
| nodepool-launcher | deployment | N |
| nodepool-builder | statefulset | N |

`nodepool-builder` requires access to at least one `image-builder` machine that is to be deployed out-of-band. Due to security limitations,
it is impossible (or at least very hard) to build Nodepool images on a pod, which is why this process must be delegated remotely to an `image-builder` machine.

> The only requirement for the `image-builder` machine is that the `nodepool-builder` is able to run Ansible tasks via SSH. Please refer to sections [Get the builder's SSH public key](#get-the-builders-ssh-public-key) and [Configuring Nodepool builder](../user/nodepool_config_repository#configuring-nodepool-builder).

> There is no assumption about the processes and toolings used to build images on the `image-builder`, except that the workflow must be driven by an Ansible playbook from the `nodepool-builder`.

## Services configuration

Configuring the Nodepool micro-services is done through the SoftwareFactory deployment's manifest. Many configuration parameters are exposed by The [SoftwareFactory Custom Resource spec](./../../config/crd/bases/sf.softwarefactory-project.io_softwarefactories.yaml).

The spec is constantly evolving during alpha development, and should be considered
unstable but the ultimate source of truth for documentation about its properties.

## Setting up providers secrets

Currently the SF Operator supports OpenStack (`clouds.yaml`) and Kubernetes (`kube.config`) configuration files. These files are used by Nodepool to manage resources on its providers.
They are managed by the SF Operator in a secret called `nodepool-providers-secrets`.

To push your configuration file(s) to Nodepool:

1. Edit your [sfconfig.yaml](./../../sfconfig.yaml) to add the paths to your configuration files:

```yaml
ansible_microshift_role_path: ~/src/github.com/openstack-k8s-operators/ansible-microshift-role
microshift:
  host: microshift.dev
  user: cloud-user
fqdn: sfop.me
nodepool:
  clouds_file: /path/to/clouds.yaml
  kube_file: /path/to/kube/config
```

2. Run sfconfig:

```sh
./tools/sfconfig nodepool-providers-secrets --update
```

3. Wait until your deployment becomes ready again:

```sh
kubectl get sf -n sf -w
```

When your deployment is ready, the provider secrets have been updated in Nodepool.

> You can also fetch the currently used configurations by running `./tools/sfconfig nodepool-providers-secrets --dump` ,
which will copy the configuration files from Nodepool into the files defined in sfconfig.yaml. Be careful not to erase
important data!

## Get the builder's SSH public key

The Nodepool builder component should be used with at least one `image-builder` companion machine.
It must have the capablility to connect via SSH to the builder machine.

Here is the command to fetch the builder SSH public key:

```sh
kubectl get secret nodepool-builder-ssh-key -n sf -o jsonpath={.data.pub} | base64 -d
```

## Accept an image-builder's SSH Host key

Once an account has been created to an `image-builder` host the `nodepool-builder` must trust the SSH Host key before being able to connect. Run the following command to initiate a SSH connection and trust the host key:

```sh
kubectl exec -it nodepool-builder-0 -c nodepool-builder -- ssh nodepool@<image-builder-hostname> hostname
```

## Using the Openshiftpods driver with your cluster

Nodepool's [Openshiftpods driver](https://zuul-ci.org/docs/nodepool/latest/openshift-pods.html) enables
Zuul to request pods from an OpenShift cluster to run jobs.

We recommend using a dedicated namespace with the driver (if you intend to spawn pods in the same OpenShift cluster than the one your deployment lives in).

The `sfconfig` CLI can automate the creation of such a namespace, and set up the associated kube config as a Nodepool secret:

```sh
sfconfig create-namespace-for-nodepool --nodepool-context my-context --nodepool-namespace nodepool-pods
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
            image: quay.io/fedora/fedora:38
```

Commit your change, review it and validate it. After a run of `config-update`, your new provider and
labels will be available in Nodepool.

## Using the Nodepool CLI

The `nodepool` command line utility is available on `nodepool-launcher` pods.

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

### How to connect on a ready node from the launcher pod

`nodepool list` is used to list all node managed by the launcher and their current status.

```sh
$ kubectl exec -ti nodepool-launcher-$uuid -c launcher -- nodepool list
```

Look for the node's IP address then from the Zuul executor pod, run:

```sh
$kubectl exec -ti zuul-executor-0 -- ssh -o "StrictHostKeyChecking no" -i /var/lib/zuul-ssh/..data/priv <user>@<ip>
Warning: Permanently added '$public_ip' (ED25519) to the list of known hosts.
$ hostname
np0000000001
```

### Accessing the Nodepool API

Nodepool exposes some [API endpoints](https://zuul-ci.org/docs/nodepool/latest/operation.html#web-interface).

For instance, to reach the `image-list` endpoint a user can access the following URL: `https://nodepool.<fqdn>/image-list`.
