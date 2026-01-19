# Upgrading Software Factory

This document provides general guidelines regarding upgrades of Software Factory with sf-operator.

!!! note
    Software Factory deployments come in many forms: as a CRD, managed through the sf-operator CLI, with external executors ...
    So these guidelines remain intentionally generic. Please adapt depending on your own situation.

1. [Pre-flight Checks](#pre-flight-checks)
1. [Pausing executors](#pausing-executors)
1. [Specific upgrade instructions](#specific-upgrade-instructions)

## Pre-flight Checks

### Check this document

Any specific steps that need to be taken to upgrade to a specific version will be documented here.

### Check the CHANGELOG

The CHANGELOG for the latest release of sf-operator will list all major changes for the latest release. This document
should explicitly mention any breaking changes, deprecations or expected component restarts.

### Run sf-operator in dry-run mode

The `--dry-run` option of the `deploy` subcommand can show you which resources would be modified and why, without actually
changing them. You can then plan accordingly.

## Pausing executors

If an upgrade requires the restart of Zuul Executor component(s), you may want to do so in a way that minimizes the risk of losing
any running jobs on the executors.

The recommended way to do so is to pause executor(s) prior to the planned upgrade so that they don't accept any new jobs,
and wait until all running jobs have been evacuated on the executor(s).

!!! warning
    Make sure you don't need Zuul to merge anything during this period!

For example, to pause an executor (assuming the proper KUBECONFIG and context):

```shell
kubectl exec zuul-executor-0 -- zuul-executor pause
```

If an executor is running jobs, their contexts can be found in `/var/lib/zuul/builds` on the executor's pod. Therefore, you can check if a paused executor is done with jobs with the following command:

```shell
kubectl exec zuul-executor-0 -- ls /var/lib/zuul/builds | wc -l
```

If that command returns 0, the executor can be restarted safely during the upgrade process.

## Run the CLI

!!! note
    This only applies to non-OLM deployments of sf-operator.

Run the CLI as you would normally to deploy your Software Factory CRD. The Software Factory components will be upgraded as needed.

## Specific upgrade instructions

### Upgrading to master

N/A - placeholder for versions as needed