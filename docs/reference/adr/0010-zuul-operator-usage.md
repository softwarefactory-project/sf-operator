---
status: approved
date: 2023-09-11
---

# Usage of the upstream zuul-operator

## Context and Problem Statement

The sf-operator, as an evolution of Software Factory, is made to ease operating Zuul and Nodepool along with providing some additional services and workflows on top of them.

We mainly target OpenShift as our deployment system of choice. Thus the sf-operator can assume
some facilities available on the target system (for instance: base set of operators such as
openshift-routes, cert-manager, monitoring, ...). Also, we want to align the sf-operator
with OpenShift security requirements and provide container images based on [UBI](https://www.redhat.com/en/blog/introducing-red-hat-universal-base-image).

The Zuul upstream project develops and maintains a k8s operator called [zuul-operator](https://opendev.org/zuul/zuul-operator).

In this ADR we provide insights on our choice to not build the sf-operator on top of the zuul-operator.

## Considered Options

### 1. Using the upstream zuul-operator:

The zuul-operator can be installed on k8s by applying some YAML manifests then it provides a CRD
to give the capability to spawn a working set of services: Zuul services, Nodepool services,
Zookeeper and a SQL server.

In this option, the sf-operator, as it needs to spawn the same components, could simply apply a
CR of the `Zuul` kind then performs the rest of actions to provide additional services and workflows on top.

Cons of this option:

  * Usage of Python and Kopf is not part of the supported choice of the [operator-framework](https://sdk.operatorframework.io/)
  * Effort to ensure compatibility with OpenShift is difficult to estimate (Current upstream CI focuses only on k8s, the operator is not validated against OpenShift)
  * Effort to ensure compatibility with images we built based on UBI is difficult to estimate (Current upstream CI uses Ubuntu based image by default)
  * Code reviews and code landing on the project might take long
  * No OLM support which brings difficulties regarding dependencies
  * OpenShift related security requirements might not be an upstream priority

Pros of this option:

  * We benefit from the upstream work of building the operator
  * We contribute to the zuul-operator project

### 2. Handling Zuul and Nodepool in the sf-operator

In this option, the sf-operator manages the deployment of Zuul and Nodepool (based on the
upstream source code).

Cons of this option:

  * We don't benefit from the upstream work of building the operator
  * We don't (or less) contribute to the zuul-operator project

Pros of this option:

  * We can build and maintain Zuul and Nodepool container images based on RedHat based distributions
    (CentOS Stream or RHEL).
  * We can focus on mainly supporting OpenShift. For instance:
    * Route and Ingress do not follow the exact same APIs.
    * CR' Controllers are well known on OpenShift. On others k8s based clusters there is no
      guaranty about the underlying system for the ingress or storage for instance.
  * We can write the sf-operator with the Go language, using libraries such as the
    [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) and benefit a larger
    community. Also, the Go language brings a light-weight type system and a good IDE integration via the
    language server which could improve our velocity.
  * We can benefit from the operator-sdk facilities and this aligns with some others
    operators developed by Red Hat folks.
  * We can benefit from OLM as one distribution vector for the sf-operator "OLM package" and let
    OLM handle installation of dependencies.
  * Building the integrations (config repository workflow, logserver, HTTPS Routes, ...) is/will
    probably be easier with this option as we can quickly adapt the base components deployments (Zuul and Nodepool).
  * We can manage to align with OpenShift security requirements. An operator
    provides cluster APIs and fonctionality extensions, so this aspect is very important when
    the operator is deployed.


## Decision Outcome

Chosen option: 2: Handling Zuul and Nodepool in the sf-operator

This choice is based on the current state of the zuul-operator project but it is subject to
change in the future.