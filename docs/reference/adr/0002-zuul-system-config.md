---
status: proposed
date: 2022-11-14
---

# Zuul System Config

## Context and Problem Statement

How to provide a config workflow's Zuul definition easier to update and prevent
SF's users to customize base jobs leading to a situation where they must be updated
manually. This situation happens with sf-config because the **config** repository
is used to store **base jobs and playbooks** and the rest of the SF configuration.

## Considered Options

* system-config Git repository
* TBD

## Decision Outcome

Chosen option: TBD

## Pros and Cons of the Options

### system-config Git repository

The sf-operator manage a Git server, dedicated to host a **system-config** repository.

The repository content is automatically provisionned to store the **config** repository
workflow:

- a base job with pre/post playbook (configuration for log storage export)
- dedicated pipelines for the **config workflow**
- config-check and config-update jobs
- system secrets (logserver, k8s, ...)

Finally, pros and cons are:

* Good, because the **system config** workflow cannot be changed by users then
  is easier to manage automatically.
* Bad, because we need to manage two config repositories **config** and **system-config**.
