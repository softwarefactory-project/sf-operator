---
status: proposed
date: 2022-11-10
---

# Config update workflow - base system

## Context and Problem Statement

This ADR is about the choice of the base system to perform actions on
a running deployment from a Zuul CI job.

We want to keep the current workflow, implemented in sf-config, where
a configuration change is proposed as a **Change** (Review, Pull Request, ...)
and validated by a Zuul CI job named: **config-check** and applied on
the running deployment via a Zuul CI job named: **config-update**.

However, the sf-config workflow relies on a nested ansible playbook executed from the
install-server which doesn't require ansible integration for Zuul job. Though we would like to
change this implementation to a regular Zuul job to improve the user experience, by removing the
need for the install server, and by providing the results directly in the Zuul build page.

In sf-config, the **config-update** Zuul job runs an ansible-playbook to
apply services configuration, to the deployed nodes, via ssh.

Applying changes is done in various ways, either by:

  - copying/updating configuration files
  - calling APIs
  - restart/reload service using systemd

On a sf-operator deployment, running on k8s, we cannot envision applying
configuration changes the same way.

## Considered Options

Here are some available options to run via the *config-update* Zuul CI job:

* Use kubectl/oc
* Use the Ansible k8s module
* Use a GO program
* Combine of Ansible k8s module + kubectl/oc + Dhall k8s

## Decision Outcome

Chosen option: "TBD", because ...

## Pros and Cons of the Options

### Use kubectl/oc

* Good, because it is the official k8s/OC client
* Neutral, can be used as simple command via the command or shell Ansible module
* Bad, not the best Ansible integration (given the fact that a Zuul CI job is an
  Ansible playbook)
* Bad, no type safety as configuration is bare YAML file

### Use the Ansible k8s module

* Good, best Ansible integration (given the fact that a Zuul CI job is an Ansible
  playbook)
* Neutral, module quality
* Bad, no type safety as configuration is bare YAML file

### Use a GO program

* Good, because of type safeties to write YAML to act on resources
* Bad, because it does not intgrate well with Ansible and we will lack of
  logs via the Zuul console

### Combine of Ansible k8s module + kubectl/oc + Dhall k8s

* Good, best Ansible integration (given the fact that a Zuul CI job is an Ansible
  playbook)
* Good, use of kubectl/oc for unsupported actions by the k8s module (such as cp
  from/to a volume)
* Good, YAML content applied by kubectl/oc or Ansible module could be built
  from a k8s dhall definition when needed to benefit type safety.
* Bad, multiple components to support, can create unnecessary complexity.
