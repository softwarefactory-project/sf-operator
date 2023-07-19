---
status: proposed
date: 2023-06-22
---

# Edge certificates management

## Context and Problem Statement

The **sf-operator** handles the **Routes** resources setup. A **Route** exposes a service externaly
to the cluster. We setup **Routes** with TLS, but currently the self signed certificate from the
**openshift-ingress** operator is used.

We need to handle the following use cases:

- CI/Dev: a self signed certificate is acceptable
- Internal deployment: **Routes** must expose a certificate signed by the internal network **CA**.
- Public deployment: **Routes** must expose a certificate signed by a public **CA**. (Software Factory 3.8 relies
  on letsencrypt or custom certificate)

The current version of the **sf-operator** relies on the **cert-manager** operator to ease certificate
issuing and this is currently only used for the internal connection between Zuul and Zookeeper.

A *POC* has been done [1] to rely on the **Route** TLS settings (Certificate, CACertificate, Key) to setup
the **Route** to use provided TLS material and not use the default from **openshift-ingress**. Note that
the *POC* uses **cert-manager** to issues a **Certificate** and loads the TLS material from the generated
**Secret**.

We currently have **Routes** resources for zuul, nodepool, and the logserver services and each service
is exposed under a sub-domain such as zuul.<fqdn>. We might need to request a certificate by service.
In this case the implementation of the solution must handle a cerificate by service instead of a
wildcard certificate for the *.<fqdn>.

## Considered Options

* 1. Provide settings via the **SoftwareFactory** resource to:

  * Optionally set custom TLS material located into a pre-provisionned **Secret**.
    The Operator then setup **Routes** with the **Secret** content.
  * Optionally set a flag to use **cert-manager** certificates.
    **sf-operator** reclaim **Certificates** to **cert-manager** which generates **Secrets** containing the TLS material.
    The Operator then setup **Routes** with the **Secret** content.
  * When no settings provided we default to **openshift-ingress** TLS default.


Pros and Cons of this option:

* Good because fit our need by provided various options to fit use cases
* Good because **cert-manager** is now part of Openshift
* Good because whether the user choose provionned **Secret** or auto managed **Secret** via **cert-manager** then
  the code remain simple by only relying on a **Secret** content with a specifc layout.
* Bad because we need to implement some code

* 2. Rely on the setup done by the Openshift cluster setup only

Pros and Cons of this option:

* Good because we fully rely on **openshift-ingress** default setup
* Bad because we are tied to external choices made by cluster operators
* Bad because it does not fit our various use cases

## Decision Outcome

Chosen option:

* 1: Provide settings via the **SoftwareFactory** resource


[1]: https://softwarefactory-project.io/r/c/software-factory/sf-operator/+/28698/