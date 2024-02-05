---
status: proposed
date: 2022-11-14
---

# Expose main.yaml to Zuul scheduler

## Context and Problem Statement

This ADR discuss how to expose the Zuul's main.yaml file to the Zuul scheduler.

Software Factory (sf-config) relies on the managesf tool and the SF resources system to
build the Zuul's main.yaml file. It seems important to continue to support this with the
sf-operator.

The chosen option must enable an easy use of managesf-configuration to generate and
expose the generated file to the zuul-scheduler container. The use of managesf-configuration
should be unified across the pod startup and the **config-update** workflow.

After the file is exposed the scheduler container, a zuul-scheduler
command (--(smart|full)-reconfigure) must be kicked on the zuul-scheduler container
to force Zuul to refresh the tenant configuration from the file. Restarting the
zuul-scheduler pod is not an option.

## Considered Options

* Via a config-map or a secret
* Via a volume (shared volume)
* Via the scheduler.tenant_config_script
* Via a sidecar container

## Decision Outcome

Chosen option: Via a sidecar container

## Pros and Cons of the Options

### Via a config-map or a secret

* Good, because the config-map can be easily updated and used as a volume mount and a file change reflect
  on the disk.
* Good, because the config-map can be updated via a **config-update** job.
* Neutral, because it lacks a solution to run managesf-configuration command.
* Bad, because the maximum size of a config-map is 1MB.

### Via a volume (shared volume)

* Good, because we are not limited by the size of the file
* Neutral, because it lacks a solution to run managesf-configuration command.
* Bad, because a volume cannot be mounted across multiple pods. A shared volume is only possible within
  the same pod.
* Bad, because a **config-update** will not be able to populate the shared volume.

### Via the scheduler.tenant_config_script

* Good, because the Zuul scheduler container can bundle the script to read/generate the tenant config.
* Bad, because the scheduler process needs to run a complex process (managesf-configuration) to
  generate the tenant config.
* Bad, because the zuul-scheduler container needs to bundle the managesf code.

### Via a sidecar container

A sidecar container (run inside the same pod) and is able to share a common volume with
the scheduler container. It can bundle a script that:

- call managesf-configuration
- copy the generated file via the shared mounted volume to the zuul-scheduler

This script can be triggered via an exec command from a **config-update** job and
during the zuul-scheduler go's controler workflow.

* Good, because we are not limited by the size of the file
* Good, because the sidecar container can bundle the managesf settings to enable the use
  of managesf-configuration tool. 
* Good, because we can share the same method to update the main.yaml file across the
  zuul-scheduler startup and **config-update** workflow.
