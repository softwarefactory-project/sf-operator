# sf-operator

**sf-operator** is the next version of [Software Factory](https://www.softwarefactory-project.io) distributed as an **OpenShift Operator**.

> Software Factory used to be a full Software Development Forge including services such as Gerrit or Etherpad, however the scope of this
new version is focused on the Continuous Integration components.

The Operator mainly manages a **SoftwareFactory** Custom Resource. This resource handles operations on the folowing services:

- [Zuul](https://zuul-ci.org/docs/zuul/latest)
- [Nodepool](https://zuul-ci.org/docs/nodepool/latest)
- Logserver (Zuul jobs artifacts storage)

The resource also deploys and maintains additional required services for Zuul and Nodepool such as:

- Zookeeper
- MariaDB

The **SoftwareFactory** CR provides a **Configuration as Code** workflow for Zuul and Nodepool. Indeed the instance must be
configured to rely on a Git repository hosted on a **Code Review System** to host non sensitive settings, such as the
[Zuul tenant configuration](https://zuul-ci.org/docs/zuul/latest/tenants.html#tenant) and the
[Nodepool configuration](https://zuul-ci.org/docs/nodepool/latest/configuration.html#configuration).

## Contacts

You can reach us on the [Software Factory Matrix channel](https://app.element.io/#/room/#softwarefactory-project:matrix.org).

## Status

The current project status is: **Alpha - DO NOT USE IN PRODUCTION**

See:

- the [CONTRIBUTING documentation](CONTRIBUTING.md) to discover how to hack on the project.
- the [CHANGELOG](CHANGELOG.md) to read about project development progress.
- the [ADRs](doc/adr/) to read about the Architecture Decision Records.

## Installation

### Prerequisites

The **sf-operator** is designed to run on Openshift and Openshift variants, thus you need an Openshift cluster to install it and make use of it.

Furthermore we package the operator via OLM so your cluster must have the [OLM system](https://olm.operatorframework.io/) running.

We currently validate the **sf-operator** on an installation of Microshift deployed via the [microshift-ansible-role](https://github.com/openstack-k8s-operators/ansible-microshift-role).

> As we are in early development phases it is recommended to use Microshift as your Openshift environment if you want to test the operator.

For more information about Microshift installation please refer to the [microshift/README.md](tools/microshift/README.md).

### Installing the CatalogSource

The **sf-operator** is packaged for OLM and is distributed by a **Catalog** we maintain. Thus to deploy the Operator you need to add
a new **CatalogSource** resource into the `olm` namespace.

Create the following CatalogSource:

```sh
cat << EOF | kubectl create -f -
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
EOF
```

Once the Catalog is defined, we can tell OLM to install the **sf-operator** by applying a **Subscription** resource:

```sh
cat << EOF | kubectl create -f -
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
EOF
```

After few seconds you should see that the operator is running:

```sh
kubectl -n operators get deployment.apps/sf-operator-controller-manager
NAME                                                READY   UP-TO-DATE   AVAILABLE   AGE
pod/sf-operator-controller-manager-65cc95995c-bgt25   2/2     Running   0          3m49s
```

Custom Resources supported by the sf-operator could be listed with the following command:

```sh
kubectl get crd -o custom-columns=NAME:.metadata.name | grep softwarefactory-project.io
logservers.sf.softwarefactory-project.io
softwarefactories.sf.softwarefactory-project.io
```

### Start a SoftwareFactory instance

Let's create a namespace where we'll reclaim a **SoftwareFactory** resource.

> Currently, the namespace must allow `privileged` containers to run. Indeed the `zuul-executor` container requires
extra priviledges due to the use of [bubblewrap](https://github.com/containers/bubblewrap).

For this example we will create **sf** namespace.

```sh
kubectl create namespace sf
kubectl label --overwrite ns sf pod-security.kubernetes.io/enforce=privileged
kubectl label --overwrite ns sf pod-security.kubernetes.io/enforce-version=v1.24
oc adm policy add-scc-to-user privileged system:serviceaccount:sf:default
```

Now we are ready to create our **SoftwareFactory** instance via OLM:

> Only one instance of a **SoftwareFactory** resource by namespace is recommanded in order to avoid resources naming collapse.

```sh
cat << EOF | kubectl -n sf create -f -
apiVersion: sf.softwarefactory-project.io/v1
kind: SoftwareFactory
metadata:
  name: my-sf
  namespace: sf
spec:
  fqdn: "sftests.com"
EOF
```

After few mimuntes, you should see a `READY` resource called `my-sf`:
```sh
kubectl -n sf get sf
NAME    READY
my-sf   true


The following `Routes` (or `Ingress`) are created:

```
kubectl -n sf get routes -o custom-columns=HOST:.spec.host

HOST
zuul.sftests.com
logserver.sftests.com
nodepool.sftests.com
```

At that point you have successfully deployed a **SoftwareFactory** instance. The Zuul web UI is accessible using the following URL: https://zuul.sftests.com

To finalize the setup you'll need to setup a **config** repository for enabling the **config as code** workflow. Please read the following guides.

## How to set your config repository
## How to configure Zuul via the config repo
## How to configure Nodepool via the config repo
## How to set the nodepool kube/config or clouds.yaml