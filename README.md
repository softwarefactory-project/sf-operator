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
  updateStrategy:
    registryPoll:
      interval: 60m
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

After few minutes, you should see a `READY` resource called `my-sf`:
```sh
kubectl -n sf get sf
NAME    READY
my-sf   true
```


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

## How to set the config repository

To setup the **Configuration As Code** workflow, a dedicated Git repository and a *bot* account must exist on **Gerrit**. See the "Requirements for the config repository on Gerrit" section below.

> Currently, the **sf-operator** only supports a **config** respository hosted on Gerrit code review system.

First, a Zuul connection to the Gerrit code review system must be defined via the **SoftwareFactory**'s Spec:

```sh
kubectl edit sf my-sf
...
spec:
  zuul:
    gerritconns:
      - name: gerrit
        username: zuul
        hostname: review.rdoproject.org
        puburl: "https://review.rdoproject.org/r"
...
```

Checkout the [CRD's OpenAPI schema](config/crd/bases/sf.softwarefactory-project.io_softwarefactories.yaml) for specification details.

Then, specify the **config** repository location:

```sh
kubectl edit sf my-sf
...
spec:
  config-location:
    base-url: "https://review.rdoproject.org/r"
    name: config
    zuul-connection-name: gerrit
...
```

Wait for the 'my-sf' resource to become *READY*:

```sh
kubectl get sf my-sf -o jsonpath='{.status}'

{"observedGeneration":1,"ready":true}
```

Once ready, the **config** repository must appear in the [Zuul's internal tenant projects list](https://zuul.sftests.com/t/internal/projects).
Furthermore, the **config-check** and **config-update** are set automatically to run the internal tenant's **check**, **gate** and **post** pipelines.

Any changes opened on the **config** repository will trigger the **config-check** job. The job validates the proposed configuration and applies them to Zuul and Nodepool.

### Requirements for the config repository on Gerrit

Zuul needs to authenticate on Gerrit, then a bot account must be set and zuul must be able to connect as SSH to Gerrit.

The Zuul SSH public key can be fetch from the **zuul-ssh-key** secret:

```sh
kubectl get secret zuul-ssh-key -o jsonpath={.data.pub} | base64 -d
```

The **config** repository must be set with specific ACLs and Labels. Zuul expects to report, trigger and merge based on specific labels values.

Here are the required labels to define in the **config** repository's *Access* settings (*meta/config*) on Gerrit:

```INI
[label "Code-Review"]
	function = MaxWithBlock
	defaultValue = 0
	copyMinScore = true
	copyAllScoresOnTrivialRebase = true
	value = -2 Do not submit
	value = "-1 I would prefer that you didn't submit this"
	value = 0 No score
	value = +1 Looks good to me, but someone else must approve
	value = +2 Looks good to me (core reviewer)
	copyAllScoresIfNoCodeChange = true
[label "Verified"]
	value = -2 Fails
	value = "-1 Doesn't seem to work"
	value = 0 No score
	value = +1 Works for me
	value = +2 Verified
[label "Workflow"]
	value = -1 Work in progress
	value = 0 Ready for reviews
	value = +1 Approved
```

Here are the required ACLs (Assuming the zuul bot user is part of the 'Service Users' group):

```INI
[access "refs/heads/*"]
label-Verified = -2..+2 group Service Users
submit = group Service Users
```

For further information check the Zuul documentation's [Gerrit section](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#gerrit).

## How to configure Zuul via the config repo
## How to configure Nodepool via the config repo
## How to set the nodepool kube/config or clouds.yaml
