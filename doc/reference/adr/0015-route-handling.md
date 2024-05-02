---
status: accepted
date: 2024-06-07
revise: 0007-edge-cert.md
---

# Route management

## Context and Problem Statement

It has been decided earlier in this ADR: [Edge cert ADR](./0007-edge-cert.md) to handle **Route** resources
and TLS (Let'sEncrypt, static certs) integration for the user. It appears that this choice might not be the right one as:

- A **Route** is only specific to OpenShift and prevent usage on Kubernetes
- A **Route** controller is custom to the Cluster and underlying infrastructure thus **Route** resources
  might need extra settings (labels, ...) to be functional.
- A **Route** might need to be amended by extra controller to handle the TLS section.
- A **Route** might not be need when a Service Type LoadBalancer and an external DNS is used.

## Considered Options

* 1. Make the **Route** optional and keep the current code:

Pros and Cons of this option:

* Good because the sf-operator provide more optionalities and functionalities
* Bad because we need to maintain more code and more doc

* 2. Remove the **Route** handling and clean extra code

Pros and Cons of this option:

* Good because, sf-operator become more agnostic regarding OpenShift or Kubernetes
* Good because we simplify the code base and documentation

## Decision Outcome

Chosen option:

* 2. Remove the **Route** handling and clean extra code