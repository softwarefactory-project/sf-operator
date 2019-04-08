A Software Factory Operator PoC
===============================

## Requirements:

* [OKD](https://github.com/openshift/origin/releases/download/v3.11.0/openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz)
* [Zuul Operator](https://github.com/TristanCacqueray/zuul-operator)

## Install

```shell
operator-sdk build 172.30.1.1:5000/myproject/sf-operator:latest
docker push 172.30.1.1:5000/myproject/sf-operator:latest

oc create -f deploy/crds/software-factory_v1alpha1_crd.yaml
oc create -f deploy/rbac.yaml
oc create -f deploy/operator.yaml
```

## Usage

```shell
$ oc apply -f - <<EOF
apiVersion: operator.softwarefactory-project.io/v1alpha1
kind: SoftwareFactory
metadata:
  name: example-sf
spec:
  fqdn: sftests.com
  provision_demo: True
EOF
softwarefactory.operator.softwarefactory-project.io/example-sf created

$ oc get pods
NAME                                         READY     STATUS             RESTARTS   AGE
example-sf-gerrit-gerrit-5d5bd95776-6zlmn    1/1       Running            0          2m
example-sf-zuul-pg-6bd7b68ccc-75xxb          1/1       Running            0          21s
example-sf-zuul-scheduler-6999b4bd58-5hqdw   1/1       Running            0          20s
example-sf-zuul-zk-0                         1/1       Running            0          22s

$ oc get svc
NAME                          TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)                                         AGE
example-sf-gerrit             ClusterIP   172.30.30.20     <none>        80/TCP,29418/TCP                                1h
example-sf-zuul-pg            ClusterIP   172.30.244.250   <none>        5432/TCP,9100/TCP,10000/TCP,2022/TCP,9187/TCP   1h
example-sf-zuul-scheduler     ClusterIP   172.30.164.205   <none>        4730/TCP                                        1h
example-sf-zuul-web           ClusterIP   172.30.16.125    <none>        80/TCP                                          36m
example-sf-zuul-zk-client     ClusterIP   172.30.115.36    <none>        2181/TCP                                        1h
example-sf-zuul-zk-headless   ClusterIP   None             <none>        2888/TCP,3888/TCP                               1h

$ curl 172.30.16.125/api/tenant/demo/projects
[{"name": "config", "connection_name": "gerrit", "canonical_name": "sftests.com/config", "type": "config"}, {"name": "demo-project", "connection_name": "gerrit", "canonical_name": "sftests.com/demo-project", "type": "untrusted"}]

$ oc get softwarefactory
NAME         AGE
example-sf   4m

$ oc delete softwarefactory example-sf
softwarefactory.operator.softwarefactory-project.io "example-sf" deleted
```
