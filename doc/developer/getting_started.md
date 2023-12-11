# Getting Started

This section covers the basics to get started with the development on the SF-Operator's code base.

## Table of Contents

1. [Requirements](#requirements)
1. [Run the operator](#run-the-operator)
1. [Access the services web UI](#access-the-services-web-ui)
1. [Delete the development deployment](#delete-the-development-deployment)
1. [To go further](#to-go-further)

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

### OpenShift

You need an OpenShift cluster or equivalent on which to run the operator and deploy resources.
The requirements for the cluster are [the ones listed in the operator section of this documentation](../operator/getting_started.md#prerequisites). We recommend however using a dedicated MicroShift instance, as it is much more flexible for hacking, and also the environment used to test and develop SF-Operator.

You can read about [how to deploy a MicroShift instance here](./microshift.md).

### Prepare development context

`sf-operator` uses the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows). Make sure that your current context is the right one for development. In this example, we are using the `microshift` context:

```sh
kubectl config current-context
# Must be microshift, if not:
kubectl create namespace sf
kubectl config set-context microshift --namespace=sf
```

Edit the [sfconfig.yaml](./../../sfconfig.yaml) configuration file to your liking, for example by setting up a custom FQDN.

Then run the `sfconfig` command:

```sh
./tools/sfconfig dev prepare
```

This command performs the following tasks:

- ensure namespace permissions
- ensure the installation of the Cert-manager and Prometheus operators
- ensure the deployment of Gerrit
- ensure the deployment of Prometheus
- ensure the checkout of the config and demo-project git repositories in the `deploy` directory
- ensure the creation of dedicated namespace for nodepool-launcher

The context is now ready to run the sf-operator using the `manager` or the `standalone` modes.

## Run the operator

To iterate on the development of the `sf-operator` you can either start the operator using:

- the `manager` mode: the is the default running mode of the operator.
  The `SoftwareFactory's` 's`CRD` must be installed into the cluster, and the operator watches
  for a `CR` to reconcile the state in the namespace.
- the `standalone` mode: does not require the installation of the `CRD`. The `controller-runtime`'s
  client is used to perform a `SofwareFactory` deployment based on `yaml` definition passed
  as parameter.

### Run the operator with the manager mode

Run the operator with the following command:

```sh
go run ./main.go --namespace sf
```

> The command does not return and wait for events to run the reconcile.

You can kill and restart this process every time you modify the code base
to see your changes applied to the deployed resources.

In another terminal, apply the `SoftwareFactory`'s `CR`:

```sh
kubectl apply -f playbooks/files/sf.yaml
```

Any change on the applied resource re-trigger the reconcile.

### Run the operator in standalone mode

Run the operator with the following command:

```sh
go run ./main.go --namespace sf standalone --cr playbooks/files/sf.yaml
```

> The command returns when the expected state is applied.

Each change to the `CR`, passed as parameter, will require a new run of the command to `reconcile` the change.


## Access the services web UI

You can verify that the services are properly exposed with Firefox (you may have to accept insecure connections when deploying with the default self-signed CA):

```sh
firefox https://zuul.<FQDN>
firefox https://gerrit.<FQDN>
firefox https://logserver.<FQDN>
firefox https://prometheus.<FQDN>
firefox https://nodepool.<FQDN>
```

## Delete the development deployment

Run the deletion with the following command:

```
./tools/sfconfig sf delete -a
```

## To go further

The run to `sfconfig prepare dev` setups a `Gerrit` instance with a [a **config** repository](../deployment/config_repository.md). This repository can be used to play with the deployment.


You may want to [run the test suite on your modifications](./testing.md).
