---
status: approved
date: 2023-10-18
---

# Nodepool builder

## Context and Problem Statement

The main deployment target for the `sf-operator` is `OpenShift`. The security concepts and requirements of `OpenShift `prevent containers to run as `root`.
A container's process runs as a random User ID and `OpenShift` default and recommended
[Security Context Constraint](https://docs.openshift.com/container-platform/4.13/authentication/managing-security-context-constraints.html#security-context-constraints-about_configuring-internal-oauth) disallows
a container to escalate its privileged to User ID 0 (`root`).

`Nodepool builder` relies on [diskimage-builder](https://opendev.org/openstack/diskimage-builder) as the default tool to build disk images. `diskimage-builder`'s elements often make
use of the `sudo` command to gain `root` access.

`Software Factory 3.8` makes use of the [dib-cmd](https://zuul-ci.org/docs/nodepool/latest/configuration.html#attr-diskimages.dib-cmd) feature to use
[virt-customize](https://www.libguestfs.org/virt-customize.1.html) as an alternative tool to `diskimage-builder`. However `virt-customize` and related library `libguestfs` might requires running
as `root`.

Given that context, building container images directly onto `OpenShift` using `Nodepool Builder` has been proven difficult. A more permissive SCC could be used for `Nodepool Builder` but
this requires that your `OpenShift`'s cluster admin allows this security exemption.


## Considered Options

### 1. disk-image builds happen on the Nodepool-builder pod

Cons of this option:

  * Nodepool-builder pod is required to run with a privileged SCC if super-users, like root, are required to build disk-images.
  * Nodepool-builder container image must include extra build tooling to accomodate various needs
  * Privileged SCC might require cluster admin permission

Pros of this option:

  * Simple setup, usable as long as extra privileged (like `root`) are not required to build disk-images.

### 2. Nodepool-builder relies on dib-cmd to run external disk-image builds

Cons of this option:

  * More complex setup as an `image-builder` machine is required

Pros of this option:

  * No assumption about the tooling needed to build images
  * No assumption about the resources needed to build images
  * No need for privileged SCC
  * Disk-image builds can still be performed on the nodepool-builder pod as long as extra privileged (like `root`) are not required for the builds

## Decision Outcome

Chosen option: 2: Nodepool-builder relies on dib-cmd to run external disk-image builds
