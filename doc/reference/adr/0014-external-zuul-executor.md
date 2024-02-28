---
status: approved
date: 2024-02-05
---

# External Zuul Executor

## Context and Problem Statement

Due to the usage of [bubblewrap](https://github.com/containers/bubblewrap) the zuul-executor
process requires a runtime environment which is not allowed by the default
`SecurityContextConstraint`s of `OpenShift`. Even if the default
can be relaxed and then set to allow the zuul-executor pod to be started, on some OpenShift
clusters such exception cannot be accepted. In this situation the `sf-operator` must
enable, for every `SoftwareFactory` deployment, the capability to connect external
zuul-executor processes.

Here are some resources raising the current issue:

* https://blog.chmouel.com/2022/01/25/user-namespaces-with-buildah-and-openshift-pipelines/
* https://github.com/containers/buildah/issues/1335
* https://lists.zuul-ci.org/archives/list/zuul-discuss@lists.zuul-ci.org/thread/C42MY2UUOP4BC2QAPJMXLE2HV4YOKUWW/

This ADR covers the choice of:

* how such external zuul-executor shoud be deployed and maintained.
* how configuration will be performed

Having external zuul-executor processes requires external access to the Zookeeper instance located inside
the control plan (on OpenShift). Zookeeper authenticates clients based on a client X509 Certificate. External
Access to Zookepper must be enabled via an OpenShift Service LoadBalancer or a NodePort.

Client certificate must be exposed as a `Secret` and used by infra automation to perform zuul-executor
processes configuration.

## Considered Options

1. Container operated by systemd, deployed via Ansible, via the automation (sf-infra) adjacent to the main deployment

Via the automation (in our case sf-infra), the control plan is deployed, then the external zuul-executor is deployed using some Ansible roles to deploy on regular host using podman + systemd.

2. Container operated by systemd, deployed via Ansible from the sf-operator

Via the automation, the control plan is deployed and it deploy the external zuul-executor using some Ansible roles on regular host using podman + systemd, using an Ansible Pod runner.

3. CRD operated by k8s, deployed via Ansible

Via the automation, the control plan is deployed, then the external zuul-executor is deployed on a k8s system allowing privileged pods. The Software Factory CRD is re-used to provide the capability to deploy only a zuul-executor StatefulSet.

4. CRD operated by k8s, deployed via kube client from the sf-operator of the main deployment

Via the automation, the control plan is deployed and it deploys the external zuul-executor using an extra go-client + kube credential of the k8s cluster.

## Decision Outcome

Chosen option: 3

## Pros and Cons of the options

From the scope of the deployment system we can compare the strategy of the proposed options.

### Ansible vs k8s

#### Deployment via ansible

* Good, work on baremetal/vm if needed
* Bad, code duplication with operator code, though it can be kept minimum if the config are pre-generated
* Bad, no auto-update on sf-operator release

#### Deployment via k8s

* Good, we can re-use the existing code to setup the service
* Bad, we need a k8s api with privileged context
* Good, auto-update on sf-operator release

### sf-infra vs sf-operator

#### Deployment via sf-infra

* Good, we already have the workflow
* Bad, we need to keep the secret in sync in the vault

#### Deployment via sf-operator

* Good, we ensure consistency
* Bad, we complexify the SF CRD to include external executor
