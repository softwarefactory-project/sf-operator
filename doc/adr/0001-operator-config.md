---
status: proposed
date: 2022-11-14
---

# SF operator configuration

## Context and Problem Statement

How the sf-operator deployment configuration is managed. This includes:

- operator' components such as Zuul, Gerrit, ...
- component's configuration, such as Zuul connections, Zuul tenant's config, Gerrit settings, ...

In sf-config, configuration is spreaded over:

- the arch.yaml file
- the config.yaml file
- the common-vars.yaml file
- the config repository


## Considered Options

* The K8S SofwareFactory Custom Resources and the config repository
* TBD

## Decision Outcome

Chosen option: TBD

## Pros and Cons of the Options

### The K8S SofwareFactory Custom Resources and the config repository

The Software Factory operator brings a **SoftwareFactory Custom Resources schema** that
define:

- the architecture (which components to enable)
- the components' configurations

We mean by *components' configurations* any configuration needed by a component that
is rational in term of definition schema complexity and size.

Configuration that is automatically generated from the config repository might be
applied to the deployment by updating the CR instance or by updating specific
resources like a configMaps.

The SoftwareFactory Custom Resource is applied via kubectl to deploy an instance
of the sf-operator. At deployment the applied resources defintion is *dumped* in
the **config** repository.

The **config** git repository and its workflow is the entry point to change the configuration
of a sf-operator deployment. The repository is hosted on a review capable code hosting
service (like the included Gerrit component) and stores the deployment configuration.

For instance:

- the dump of the sf-operator CR (as applied via kubectl)
- the SF resources definition (projects, Gerrit repositories, ...)
- the Zuul tenants as flat files (outside the SF resources)
- the Gerrit replication definition
- the Nodepool configuration
- Any additional Zuul config (jobs, roles, ...)

The **config's workflow** is built based on Zuul jobs:

- **config-check**: to validate a configuration change
- **config-update**: to apply a configuration change

The **config-update** job is capable of acting on the current deployment to update:

- the architecture
- the components' configurations

Finally, regarding this option the pros/cons are:

* Good, because we keep a workflow almost similar to sf-config
* Good, because we catch the whole configuration in the config repository
* Good, because configuration is less dispatched over multiple files
* Neutral, because it could brings the capability to remove critical config/component
* Bad, because not isofunctional with sf-config
