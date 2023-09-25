# Getting Started with a deployment

## Table of Contents

1. [Prerequisites](#prerequisites)
1. [Installation](#installation)
1. [Next steps](#next-steps)

## Prerequisites

For simplicity's sake, we will refer to a Zuul-based CI infrastructure that can be deployed with SF-Operator as a **"Software Factory"**.

In order to deploy a Software Factory with the SF-Operator on an OpenShift cluster, you will need the following:

1. The SF-Operator must be installed on the cluster. Ask your cluster admin whether it is the case or not; or if you are allowed to install it on the cluster, follow the [Operator's installation steps in the documentation](../operator/getting_started.md).
1. A valid kubeconfig file, for a user with enough rights to create a namespace (optional) and enable privileged [SCCs](https://docs.openshift.com/container-platform/4.13/authentication/managing-security-context-constraints.html) on the target namespace.

## Installation

We recommend using a dedicated namespace to deploy your Software Factory. Furthermore, only one instance of a Software Factory must be deployed per namespace due to name and label collisions.

> Currently, the namespace must allow `privileged` containers to run. Indeed the `zuul-executor` container requires
extra privileges because of [bubblewrap](https://github.com/containers/bubblewrap).

For this example we will create **sf** namespace. The last three commands below configure privileged access on this namespace; modify the commands accordingly if using an existing namespace.

> Note that these commands might need to be run by a user with enough privileges to create and modify namespaces and policies.

```sh
kubectl create namespace sf
kubectl label --overwrite ns sf pod-security.kubernetes.io/enforce=privileged
kubectl label --overwrite ns sf pod-security.kubernetes.io/enforce-version=v1.24
oc adm policy add-scc-to-user privileged system:serviceaccount:sf:default
```

Create a **SoftwareFactory** Custom Resource as a file named `my-sf.yaml`. Here is a minimal example that uses a default configuration; only the FQDN of the services is mandatory:

```yaml
apiVersion: sf.softwarefactory-project.io/v1
kind: SoftwareFactory
metadata:
  name: my-sf
  namespace: sf
spec:
  fqdn: "sftests.com"
```

then create the resource with:

```sh
kubectl -n sf create -f my-sf.yaml
```

After some time, a resource called `my-sf` will appear as READY:
```sh
kubectl -n sf get sf
NAME    READY
my-sf   true
```


The following `Routes` (or `Ingresses`) are created:

```
kubectl -n sf get routes -o custom-columns=HOST:.spec.host

HOST
zuul.sftests.com
logserver.sftests.com
nodepool.sftests.com
```

At that point you have successfully deployed a **SoftwareFactory** instance. You can access the Zuul Web UI at https://zuul.sftests.com .

## Next steps

To finalize the deployment, you'll need to [set up a **config** repository](./config_repository.md).