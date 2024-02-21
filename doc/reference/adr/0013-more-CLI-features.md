---
status: accepted
date: 2024-01-26
---

# more CLI subcommands

## Context and Problem Statement

By the time this ADR gets merged, the CLI overhaul will be mostly complete for the developer's experience. With our operations experience, let's also add subcommands that will help our day-to-day maintenance.

## Considered Action

Proposed subcommand hierarchy:

### zuul

Add the following "proxies" for the most often used commands of the Zuul CLIs:

#### zuul-admin

* **create-auth-token [args]**
* **delete-pipeline-state [args]**

#### zuul-scheduler

* **tenant-reconfigure [args]**
* **smart-reconfigure**
* **full-reconfigure**

And add the following new subcommands:

* **get connection-ssh-key** - this command will download Zuul's public key that can be used to configure connections (gerrit, etc). Basically similar to `nodepool get builder-ssh-key`.
* **inject-key [--build-id XXX --autohold-id YYY] /path/to/pubkey** - inject an ssh pub key on every node of a given held build (can be retrieved by build ID or autohold ID). Outputs the SSH command(s) to connect to each node.
* **restart** - this command will have the same effect as running the playbook at https://softwarefactory-project.io/r/plugins/gitiles/software-factory/sf-ops/+/refs/heads/master/maintenance/zuul-restart.yml

#### zuul-client

We shouldn't proxy this CLI to avoid issues with handling arguments, especially when reading from a local file is required (the cli config for example). Instead, let's provide a utility to generate a CLI config:

* **create client-config [--auth-token-ttl 3600]** will generate a zuul-client CLI config file with each tenant provisioned. By default, generate a config without a JWT. If the token TTL flag is provided with a value in seconds, add auth tokens to the config with the provided expiry time.

### zookeeper

* **get queue** - dumps the queue state for debugging
* **get nodes** - dumps the nodes state for debugging

## Decision Outcome

Add the proposed subcommands to the CLI.

### Consequences

* Good, because these commands are useful for day-to-day maintenance.