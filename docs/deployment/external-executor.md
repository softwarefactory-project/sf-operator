# External Zuul Executor

## Control plane

The Zuul executor must be disabled on the control plane by setting `enabled` to `false` in the `spec.zuul.executor` section. Furthermore, the `k8s-api-url` and
the `logserver-host` setting must be set in the `spec.config-location` section.

```yaml
apiVersion: sf.softwarefactory-project.io/v1
kind: SoftwareFactory
metadata:
  name: my-sf
  namespace: sf
spec:
  fqdn: "sfop.me"
  config-location:
    k8s-api-url: "https://<control-plane-cluster-api-url>:6443"
    logserver-host: "<hostname-or-ip-of-logserver-sshd-service>"
    ...
  zuul:
    gerritconns:
      ...
    executor:
      enabled: false
```

The zuul executor component(s) require access to the following control plane services:

- Zookeeper (2281/TCP)
- The system-config git server (9418/TCP)
- The logs server (2222/TCP)

A way to enable ingress on such a service is to use a Service Resource of type LoadBalancer:

```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: zookeeper-lb
spec:
  ports:
    - name: zookeeper-2281
      port: 2281
      protocol: TCP
      targetPort: 2281
  selector:
    statefulset.kubernetes.io/pod-name: zookeeper-0
    app: sf
    run: zookeeper
  type: LoadBalancer
---
apiVersion: v1
kind: Service
metadata:
  name: git-server-ro-lb
spec:
  ports:
    - name: git-server-port-9418
      port: 9418
      protocol: TCP
      targetPort: 9418
  selector:
    statefulset.kubernetes.io/pod-name: git-server-0
  type: LoadBalancer
---
apiVersion: v1
kind: Service
metadata:
  name: logserver-lb
spec:
  ports:
    - name: logserver-2222
      port: 2222
      protocol: TCP
      targetPort: 2222
  selector:
    statefulset.kubernetes.io/pod-name: logserver-0
  type: LoadBalancer
```

## Executor

The `SoftwareFactory`'s CR to deploy only the `zuul-executor` component (on a cluster allowing the `Privileged` SCC) must be as follows:

```yaml
apiVersion: sf.softwarefactory-project.io/v1
kind: SoftwareFactory
metadata:
  name: my-ext-ze
spec:
  fqdn: "sfop.me"
  zuul:
    executor:
      standalone:
        controlPlanePublicZKHostname: "<hostname-or-ip-of-zookeeper-service>"
        controlPlanePublicGSHostname: "<hostname-or-ip-of-gitserver-service>"
        publicHostname: <hostname-or-ip-of-executor>
```

Some secrets must be synchronized between the control plane's namespace and the zuul-executor namespace. Here is the list
of secrets that must be synchronized:

- ca-cert
- zookeeper-client-tls
- zuul-ssh-key
- zuul-keystore-password

The following command shows how to synchronize a secret:

```sh
kubectl --config ~/.kube/control-plan.yaml get secrets ca-cert -o json | \
  jq --arg name ca-cert '. + {metadata: {name: $name}}' | \
  kubectl --config ~/.kube/external-ze-01.yaml apply -n ext-ze -f
```

Zuul's connection definition must be similar in both Custom Resources, and the connection's secrets must be synchronized between
the control plane's namespace and the zuul-executor namespace.

The control plane `zuul-web` must be able to access the `zuul-executor` component(s) finger port 7900.
To do so, the following service can be defined:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: zuul-executor-headless-hp
spec:
  ports:
    - name: zuul-executor-7900
      port: 7900
      protocol: TCP
      targetPort: 7900
  selector:
    app: sf
    run: zuul-executor
  type: LoadBalancer
```