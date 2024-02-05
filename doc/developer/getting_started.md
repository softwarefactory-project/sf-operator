# Getting Started

This section covers the basics to get started with the development on the SF-Operator's code base.


1. [Requirements](#requirements)
1. [Run the operator](#run-the-operator)
1. [Access the services web UI](#access-the-services-web-ui)
1. [Delete the development deployment](#delete-the-development-deployment)
1. [Next actions](#next-actions)

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

[Create a sf-operator configuration file](../reference/cli/index.md#config) to your liking:

```sh
sf-operator init config --dev > /path/to/sfcli.yaml
```

Then edit it as necessary, for example by setting up a custom FQDN. Below is an example you can copy and edit.

??? abstract "CLI configuration example"

    ```yaml title="sfcli.yaml"
    --8<-- "sfcli.yaml"
    ```

Then run the `sf-operator` command:

```sh
sf-operator --config /path/to/sfcli.yaml dev create demo-env
```

[This command](./../reference/cli/index.md#create-demo-env) performs the following tasks:

- ensure the deployment of a test Gerrit instance
- ensure the checkout of the `config`, `demo-tenant-config` and `demo-project` git repositories in the directory of your choosing (defaults to `deploy`)
- ensure the configuration of a test openshiftpods provider for nodepool

The context is now ready to run the sf-operator using the `manager` or the `standalone` modes.

## Run the operator

To iterate on the development of the `sf-operator` you can either start the operator using:

- the `manager` mode: the is the default running mode of the operator.
  The `SoftwareFactory's` 's`CRD` must be installed into the cluster, and the operator watches
  for a `CR` to reconcile the state in the namespace.
- the `standalone` mode: does not require the installation of the `CRD`. The `controller-runtime`'s
  client is used to perform a `SofwareFactory` deployment based on `yaml` definition passed
  as parameter.

=== "manager mode"

    First, apply the `SoftwareFactory`'s `CR`:
    
    ```sh
    kubectl apply -f playbooks/files/sf.yaml
    ```
    
    then run the operator with the following command:
    
    ```sh
    sf-operator --namespace sf operator
    ```

    !!! note
        The command does not return and wait for events to run the reconcile.
    
    You can kill and restart this process every time you modify the code base
    to see your changes applied to the deployed resources.
    
    Any change on the applied `SofwareFactory`'s `CR` will re-trigger the reconcile.

=== "standalone mode"

    Run the operator with the following command:
     
    ```sh
    sf-operator --namespace sf apply --cr playbooks/files/sf.yaml
    ```
     
    !!! note
        The command returns when the expected state is applied.
     
    Each change to the `CR`, passed as parameter, will require a new run of the command to `reconcile` the change.


## Access the services web UI

You can verify that the services are properly exposed with Firefox (you may have to accept insecure connections when deploying with the default self-signed CA):

```sh
firefox https://<FQDN>/zuul
firefox https://<FQDN>/logs/
firefox https://<FQDN>/nodepool/api/image-list
firefox https://<FQDN>/nodepool/builds/
firefox https://gerrit.<FQDN>
```

## Delete the development deployment

Wipe your deployment by running:

```sh
sf-operator SF wipe --all
```

## Next actions

Now that you have a testing environment set up, you may want to [run the test suite on your modifications](./testing.md).
