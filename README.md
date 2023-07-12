# sf-operator

**sf-operator** is the next version of [Software Factory](https://www.softwarefactory-project.io).

It is an **OpenShift Operator** capable of deploying and maitaining Software Factory's services.

## Contacts

You can reach us on [the Software Factory Matrix channel](https://app.element.io/#/room/#softwarefactory-project:matrix.org).

## Status

The current project status is: **Alpha - DO NOT USE IN PRODUCTION**

See the [CONTRIBUTING documentation](CONTRIBUTING.md) to discover how to hack on the project.

See the [CHANGELOG](CHANGELOG.md) to read about project development progress.

## ADR

Architecture Decision Records are available as Markdown format in *doc/adr/*.

To add a new decision:

1. Copy doc/adr/adr-template.md to doc/adr/NNNN-title-with-dashes.md, where NNNN indicates the next number in sequence.
2. Edit NNNN-title-with-dashes.md.

More information in the [ADR's README](doc/adr/README.md).

## Installation

### Prerequisites
The sf-operator is designed to support Openshift and Openshift variants, thus you need an Openshift cluster to install it and make use of it.
Furthermore we package the operator via OLM so your cluster must have the OLM system running.

We currently test the sf-operator on an installation of Microshift deployed via [microshift ansible role](https://github.com/openstack-k8s-operators/ansible-microshift-role.

As we are in early development phase it is recommended to use Microshift as your Openshift environment.

For more information about microshift installation please refer to the file **tools/microshift/README.md**

### Setting Software Factory Catalog Source

First, you need to add a Catalog Source resource entry by defining a new Catalog.
A Catalog is a registry where kubernetes can then can get the necessary resources for Software Factory's Operator, like CSVs, CRD, and other operator resources.

Create the following Catalog Source:
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

Once the Catalog is defined, we can tell OLM to install the sf-operator by applying a Subscription resource:
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
After few seconds you should see that the sf-operator is running:
```sh
kubectl -n operators get all
NAME                                                  READY   STATUS    RESTARTS   AGE
pod/cert-manager-5f9459897f-xkpz9                     1/1     Running   0          3m44s
pod/cert-manager-cainjector-64756cdff4-d57rj          1/1     Running   0          3m44s
pod/cert-manager-webhook-857d96bc69-wm8jl             1/1     Running   0          3m44s
pod/sf-operator-controller-manager-65cc95995c-bgt25   2/2     Running   0          3m49s

NAME                                                     TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
service/cert-manager                                     ClusterIP   10.43.246.79    <none>        9402/TCP   3m52s
service/cert-manager-webhook                             ClusterIP   10.43.184.247   <none>        443/TCP    3m51s
service/cert-manager-webhook-service                     ClusterIP   10.43.249.183   <none>        443/TCP    3m46s
service/sf-operator-controller-manager-metrics-service   ClusterIP   10.43.2.166     <none>        8443/TCP   3m55s

NAME                                             READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/cert-manager                     1/1     1            1           3m46s
deployment.apps/cert-manager-cainjector          1/1     1            1           3m46s
deployment.apps/cert-manager-webhook             1/1     1            1           3m46s
deployment.apps/sf-operator-controller-manager   1/1     1            1           3m51s

NAME                                                        DESIRED   CURRENT   READY   AGE
replicaset.apps/cert-manager-5f9459897f                     1         1         1       3m46s
replicaset.apps/cert-manager-cainjector-64756cdff4          1         1         1       3m46s
replicaset.apps/cert-manager-webhook-857d96bc69             1         1         1       3m46s
replicaset.apps/sf-operator-controller-manager-65cc95995c   1         1         1       3m51s
```

The Custom Resources supported by the sf-operator must be listed with the following command:
```sh
kubectl get crd -o custom-columns=NAME:.metadata.name | grep softwarefactory-project.io
logservers.sf.softwarefactory-project.io
softwarefactories.sf.softwarefactory-project.io
```
You should see the above two entries, which means that your kubernetes cluster now knows what a Software Factory resource is.

It means that your Kubernetes cluster now knows how to instanciate a Software Factory resource.

### Start Software Factory

First create the namespace space where we want Software Factory to be, with the right security permissions.
For this example we will create **sf** namespace.
```sh
kubectl create namespace sf
kubectl config set-context microshift --namespace=sf
kubectl label --overwrite ns sf pod-security.kubernetes.io/enforce=privileged
kubectl label --overwrite ns sf pod-security.kubernetes.io/enforce-version=v1.24
oc adm policy add-scc-to-user privileged -z default
oc adm policy add-scc-to-user privileged system:serviceaccount:sf:default
```

Now we are ready to create our Software Factory instance, you can find an example of a Software Factory manifest file at **config/samples/sf_v1_softwarefactory.yaml**
```yaml
# Minimal Configuration for Software Factory Operator
apiVersion: sf.softwarefactory-project.io/v1
kind: SoftwareFactory
metadata:
  name: my-sf
  namespace: sf
spec:
  fqdn: "sftests.com"
```

Now, to create a Software Factory Instance run the following:
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

After a successful creation you can check it with:
```sh
kubectl -n sf get sf
NAME    READY
my-sf   true

kubectl -n sf get all
...
...

kubectl -n sf get routes -o custom-columns=HOST:.spec.host
HOST
logserver.sftests.com
nodepool.sftests.com
zuul.sftests.com
zuul.sftests.com
```

Now go to your browser and paste the above links.

At that point you have successfully deployed a Software Factory instance. To finalize the setup you need to setup a "config" repository for enabling the config as code workflow.

## How to set your config repository
## How to configure Zuul via the config repo
## How to configure Nodepool via the config repo
## How to set the nodepool kube/config or clouds.yaml