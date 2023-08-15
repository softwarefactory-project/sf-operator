---
status: proposed
date: 2023-06-24
---

# Config check and update jobs implementation

## Context and Problem Statement

The service provides a way to update the Zuul and Nodepool configurations through code review:
A proposed change is validated with the "config-check" job, and once approved it is applied
with the "config-update" job.

The problem is that these jobs may be used to setup the initial nodepool provider, and thus
they need a dedicated place to run.

## Considered Options

### 1. config check is performed by the executor through ansible

Pros:
- Synchronous in the job playbook, e.g. simpler.
- Checks could be performed in parallel by using two Zuul jobs.

Cons:
- Nodepool validation needs to be installed in the zuul-executor.
- How to handle the zuul.conf, especially without access to zookeeper.

### 2. crd config check that is applied by the executor

Pros:

- Re-use the zuul.conf from the production
- Checks are performed in parallel
- Can be used standalone, e.g. to validate ad-hoc config
- Already implemented in topic:gsgfsg
- Uses zuul and nodepool containers images that are deployed by the operator, so we don't need to worry about installing tooling at the right version on the executor

Cons:

- Adds an extra control loop with its own resource lifecycle (config-map && jobs).
- Requires a lot of code to implement the controller which can bring unexpected bugs.
- Creates resources in the productions namespace.
- The production configs are used as-is for testing, we need to ensure that running zuul or nodepool with their respective config-check options won't mess with the production zookeeper. This can be mitigated however by adding logic in the controllers to generate dummy configs with zk data expunged; we would use these dummy configs instead for testing.

## Decision Outcome

Chosen option, 1, because it comes out best.
