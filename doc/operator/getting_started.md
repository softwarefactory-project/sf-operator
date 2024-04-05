# Getting Started with the SF Operator


1. [Prerequisites](#prerequisites)
2. [Installing the Operator](#installing-the-operator)
4. [Deinstallation the Operator](#deinstalling-the-operator)
5. [Next steps](#next-steps)

## Prerequisites

In order to install the SF Operator on OpenShift, you will need:

1. An OpenShift cluster (obviously). It is possible to use a MicroShift instance instead; in that case see the [prerequisites and/or how to deploy MicroShift in the developer's documentation](../developer/microshift.md).
1. [OLM](https://olm.operatorframework.io/) running on your cluster. For most flavors of OpenShift [this is already the case](https://docs.openshift.com/container-platform/4.13/operators/understanding/olm/olm-understanding-olm.html#olm-overview_olm-understanding-olm).
1. The community operators CatalogSource, to handle operator dependencies for SF-Operator. For most standard installations of OLM, [this CatalogSource is already installed](https://operatorhub.io/how-to-install-an-operator#How-do-I-get-Operator-Lifecycle-Manager?).
1. A valid kubeconfig file, for a user with enough permissions to create a CatalogSource and a Subscription Custom Resources, on the `olm` and `operators` namespaces respectively.
1. The [kubectl utility](https://kubernetes.io/docs/tasks/tools/#kubectl), to apply and create new resources on the OpenShift cluster. It is also required to use `restore` functionality.

## Installing the operator

### Create the CatalogSource

The **sf-operator** is packaged for OLM and is distributed by a **Catalog** we maintain. Thus to deploy the Operator you need to add
a new **CatalogSource** resource into the `olm` namespace.

Create the following CatalogSource resource; save it in a file named `sf-catalogsource.yaml`:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: sf-operator-catalog
  namespace: olm
spec:
  sourceType: grpc
  image: quay.io/software-factory/sf-operator-catalog:latest
  displayName: sf-operator
  publisher: softwarefactory-project.io
  updateStrategy:
    registryPoll:
      interval: 60m
```

Then create the resource on the cluster:

```sh
kubectl create -f sf-catalogsource.yaml
```

Once the Catalog is defined, we can tell OLM to install the **sf-operator** by applying a **Subscription** resource. Create `sf-subscription.yaml` with the following content:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: sf-operator-sub
  namespace: operators
spec:
  channel: preview
  name: sf-operator
  source: sf-operator-catalog
  sourceNamespace: olm
```

Then create the resource on the cluster:

```sh
kubectl create -f sf-subscription.yaml
```

After a few seconds you can ensure that the operator is running:

```sh
kubectl -n operators get deployment.apps/sf-operator-controller-manager
NAME                                                READY   UP-TO-DATE   AVAILABLE   AGE
pod/sf-operator-controller-manager-65cc95995c-bgt25   2/2     Running   0          3m49s
```

You can also confirm that the operator's Custom Resource Definitions (CRD) were installed properly:

```sh
kubectl get crd -o custom-columns=NAME:.metadata.name | grep softwarefactory-project.io
softwarefactories.sf.softwarefactory-project.io
```

Note that the SF-Operator OLM package depends on the following operators:

* [cert-manager](https://cert-manager.io)
* [prometheus-operator](https://prometheus-operator.dev)

Congratulations, the SF Operator is now running on your cluster!

## Deinstalling the operator

For further details on how to remove an operator, see [OLM's upstream documentation](https://olm.operatorframework.io/docs/tasks/uninstall-operator/).

## Next steps

 Your next step is to [deploy a Zuul-based CI with the operator](../deployment/getting_started.md).
