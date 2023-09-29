# Getting Started

This section covers the basic tools and the testing environment required to start working on SF-Operator's code base.

## Table of Contents

1. [Requirements](#requirements)
1. [Deploy test resources](#deploy-test-resources)
1. [Run the operator in dev mode](#run-the-operator-in-dev-mode)
1. [Next steps](#next-steps)

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

Then run the `sfconfig` command to deploy a SoftwareFactory resource, a companion Gerrit service 
preconfigured to host the deployment's config repository and a demo repository, and a companion
Prometheus:

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