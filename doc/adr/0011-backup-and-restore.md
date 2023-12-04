---
status: proposed
date: 2023-10-18
---

# Backup and restore sf-operator

## Context and Problem Statement

Software Factory' services deployed by the `sf-operator`, such as Zuul,
Zookeeper, MariaDB and others, generates critical state data.
This state data are for instance the Zuul projects's key pairs, or
the MariaDB content.

We should be able to continuously gather backups of a running deployment
and when needed to deploy a Software Factory via the sf-operator by restoring a backup.

This ADR discuss how to provide a mechanism to backup and restore critical state data.

Specifically we'll explore proposals regarding these aspects:

* What are the relevant data to backup and restore
* What is the workflow to backup and restore (such as the tooling)
* What is the form of the backup (such the data format)

## The backup and restore process requirements

The backup and restore process needs to:

* Save all resources related to the `sf-operator` such as Secrets, Config maps, etc.
* Save existing data from attached PVs related to the service. Most important
  is to have backup of Zookeeper and MariaDB database
* Should also include config maps (optionaly)
* Should make a backup of SoftwareFactory Custom Resource

Normally, in the Kubernetes world, the basic backup can be done by using
a simple solution:

* Create a PV snapshot - based on that [doc](https://kubernetes.io/docs/concepts/storage/volume-snapshots/)
* Create a secret backup via a shell command or Ansible playbook

and that's it. The config map does not need to be recreated because
we are using an operator that can recreate other required resources
automatically. However, the assumption is that no backup is required
for resource type. If the configmap needs to be included in the
backup, it can also be done by using a shell script, but maybe it
would be better to use an already-prepared tool that will handle all those things.
All the above things can be done as a user without special privileges, like
`cluster_admin` role, which is a requirement for most Kubernetes backup solutions.

Besides making backup and restore of Kubernetes resources, procedure should
also include:

* Repopulate secrets and trigger service/statefulsets/daemonsets/etc. restart
* Restore Zookeeper PV data and restart the Zookeeper
* Restore MariaDB PV data and restart MariaDB

The `sf-config` project has already done such a workflow, so based on that,
we can port functionality into the `sf-operator`.

## Considered Options

### shell script {#shell}

This solution will make a backup by using `kubectl` or `oc` binary for making
a PV snapshot or copy the content stored on the PV to a specified location for
each service/statefulsets/daemonsets/etc.. Later, such data, that has been copied
to a specified location, can be archived by a backup tool, e.g., [bup tool](https://github.com/bup/bup)

### Ansible playbook {#ansible}

This solution would include a bundle of roles and tasks that will
do a backup and/or restore all of the Software Factory CR resources.
Similar to the [shell](#shell) solution, the resource backups (like data stored
on the PV or secrets) will be archived by using the backup tool (for
example [bup tool](https://github.com/bup/bup)) and stored in a specific location.

### Add feature into the sf-operator binary {#binary}

The `sf-operator` project got a CLI binary called `sf-operator` which is
have many options that deploy and configure the Software Factory on the
Kubernetes like environment.
By choosing that option, new option like:

* Backup
* Restore

would be added to the binary.
That solution will be not using Ansible on the backgroud, so by choosing it,
we don't need to remember to install the Ansible collection, because the
`sf-config` Go binary would include the necessary library.

## Decision Outcome

The current best solution would be to add a feature to the sf-operator [binary](#binary).
That binary, which includes many features and adds a backup solution, would
be a good way to have one "tool" to make all jobs around'sf'-operator' deployment.
The binary will include all necessary libraries, so we don't have any issues like
with Ansible, that there can be a missing Ansible collection, so the backup
would not be done.

Another good solution would be the [Ansible](#ansible) one.
We already have an Ansible-base playbook that executes backups and
restore (including all mechanisms around services like stop service,
restore data, propagate data, etc.), but that solution needs to be ported to
be used in the Kubernetes.

Both solutions does not need to have a Kubernetes cluster admin privileges, and
we can dump the whole data to the local directory, where we can use previously used tools
for making backups (for example, bup).
