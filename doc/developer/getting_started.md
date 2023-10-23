# Getting Started

This section covers the basic tools and the testing environment required to start working on SF-Operator's code base.

## Table of Contents

1. [Requirements](#requirements)
1. [Deploy test resources](#deploy-test-resources)
1. [Run the operator in dev mode](#run-the-operator-in-dev-mode)
1. [Next steps](#next-steps)
1. [Experiment a deployment with the standalone mode](#experiment-a-deployment-with-the-standalone-mode)

## Requirements

### Tools

You will need to install the following tools on your dev machine:

- kubectl
- git
- git-review
- golang # >= 1.19
- make
- ansible-core
- jq

The following tools are not mandatory, but they will make your life much easier:

- python-tox
- python-kubernetes
- skopeo
- buildah

The [operator-sdk](https://sdk.operatorframework.io/) is required to generate/update the OLM bundle, or
when a new `CRD` needs to be added to the operator. You can install it with `make`:

```sh
make operator-sdk
```

### OpenShift

You need an OpenShift cluster or equivalent on which to run the operator and deploy resources.
The requirements for the cluster are [the ones listed in the operator section of this documentation](../operator/getting_started.md#prerequisites). We recommend however using a dedicated MicroShift instance, as it is much more flexible for hacking, and also the environment used to test and develop SF-Operator.

You can read about [how to deploy a MicroShift instance here](./microshift.md).

## Deploy test resources

With `sfconfig`, you can quickly deploy a demo deployment consisting of the following:

* a SoftwareFactory resource (Zuul, Nodepool, Log server and backend services)
* a companion Gerrit service hosting:
    * the deployment's config repository
    * a demo repository
* a companion Prometheus instance for monitoring

The operator will automatically use the current context in your kubeconfig file
(i.e. whatever cluster `kubectl cluster-info` shows).
Make sure that your current context is the right one for development. In this example, we are using
the microshift context:

```sh
kubectl config current-context
# Must be microshift, if not:
kubectl create namespace sf
kubectl config set-context microshift --namespace=sf
```

Edit the [sfconfig.yaml](./../../sfconfig.yaml) configuration file to your liking, for example by setting up a custom FQDN.

Then run the `sfconfig` command:

```sh
go run ./cli/sfconfig
```

You can monitor the deployment's advancement by running

```sh
kubectl get pods -w -n sf
```

The deployment is ready when every pod is either in status "Completed" or "Running".

## Run the operator in dev mode

To run the operator locally, simply do

```sh
go run ./main.go --namespace sf
```

You can kill and restart this process every time you modify the code base
to see your changes applied to the deployed resources.

## Next Steps

You can verify that the services are properly exposed with Firefox (you may have to accept insecure connections when deploying with the default self-signed CA):

```sh
firefox https://zuul.<FQDN>
firefox https://gerrit.<FQDN>
firefox https://logserver.<FQDN>
firefox https://prometheus.<FQDN>
firefox https://nodepool.<FQDN>
```

Next, you may want to [run the test suite on your modifications](./testing.md).

## Experiment a deployment with the Standalone mode

The purpose of this mode is to experiment a Software Factory deployment without the need
to get the `sf-operators' CRDs` installed on the cluster. CRDs installation requires the `cluster-admin`
right or the `sf-operator` being installed by your cluster's admin or via `OLM`.

For instance, you might want to first experiment with a sandbox deployment of Software Factory before requesting
an installation of the sf-operator to your cluster's administrator.

To experiment with a deployment (assuming a valid `kube config` file and the right `context` set), run the following command:

> `sf` namespace must have been created prior to the command below.

```sh
go run ./main.go standalone --cr config/samples/sf_v1_softwarefactory.yaml --namespace sf
```

Each change to the `CR`, passed as parameter, will require a new run of the command to `reconcile` the change.

To go further, with that deployment, please refer to [set up a **config** repository](../deployment/config_repository.md).

### Delete the sandbox deployment

This deployment mode creates an owner `ConfigMap` Resource that can deleted to trigger the deletion
of all SoftwareFactory's Resources created by the `standalone` command.

```
kubectl -n sf delete cm sf-standalone-owner
```

Then, delete the `PVCs`, with:

```
./tools/sfconfig sf delete --pvcs
```