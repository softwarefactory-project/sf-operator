# Getting Started with a deployment


1. [Prerequisites](#prerequisites)
1. [Installation](#installation)
1. [Next steps](#next-steps)

## Prerequisites

For simplicity's sake, we will refer to a Zuul-based CI infrastructure that can be deployed with SF-Operator as a **"Software Factory"**.

In order to deploy a Software Factory with the SF-Operator on an OpenShift cluster, you will need the following:

1. The SF-Operator must be installed on the cluster. Ask your cluster admin whether this is the case; or if you are allowed to install it on the cluster, follow the [Operator's installation steps in the documentation](../README.md#install).
1. A valid kubeconfig file, for a user with sufficient rights to create a namespace (optional) and enable privileged [SCCs](https://docs.openshift.com/container-platform/4.13/authentication/managing-security-context-constraints.html) on the target namespace.

## Installation

We recommend using a dedicated namespace to deploy your Software Factory. Furthermore, only one instance of a Software Factory can be deployed per namespace due to name and label collisions.

!!! note
    Currently, the namespace must allow `privileged` containers to run. Indeed the `zuul-executor` container requires
    extra privileges because of [bubblewrap](https://github.com/containers/bubblewrap).

!!! note
    The `zuul-executor` deployment can be disabled via the `CRD`. Doing so, the privileged SCC is not required. The `zuul-executor` component can be
    deployed externally to the control plane where privileged SCC is allowed.
    See the section [External executor](./external-executor.md).

In this example we will create a dedicated namespace called **sf**. Then the next command below configures privileged access on this namespace; modify the command as needed if using a different namespace.

!!! note
    Note that these commands might need to be run by a user with enough privileges to create and modify namespaces and policies.

```sh
kubectl create namespace sf
oc adm policy add-scc-to-user privileged system:serviceaccount:sf:default
```

Create a **SoftwareFactory** Custom Resource as a file named `my-sf.yaml`. Here is a minimal example that uses a default configuration; only the base FQDN for the services is mandatory:

```yaml
apiVersion: sf.softwarefactory-project.io/v1
kind: SoftwareFactory
metadata:
  name: my-sf
  namespace: sf
spec:
  fqdn: "sfop.me"
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

The `sf-operator` does not handle the `Route/Ingress` installation.

The following resource can be applied in the namespace to set up a `Route` and redirect
traffic to the gateway service.

```yaml
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: gateway
  namespace: sf
spec:
  host: sfop.me
  path: /
  port:
    targetPort: 8080
  to:
    kind: Service
    name: gateway
  tls:
    insecureEdgeTerminationPolicy: Redirect
    termination: edge
```

Once the "Route/Ingress" resource is up, here is the list of available endpoints:

- https://sfop.me/zuul
- https://sfop.me/logs
- https://sfop.me/nodepool/api/image-list
- https://sfop.me/nodepool/builds

At that point, you have successfully deployed a **SoftwareFactory** instance. You can access the Zuul Web UI at https://sfop.me/zuul.

## Next steps

To finalize the deployment, you'll need to [set up a **config** repository](./config_repository.md).
