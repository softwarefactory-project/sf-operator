# Developer HOWTOs

This document lists tips and methods for performing tasks that may be useful to a developer.


1. [How to run helper services](#how-to-run-helper-services)
2. [How to open a review on the test Gerrit](#how-to-open-a-review-on-the-test-gerrit)
3. [How to hack the hidden system-config repository](#how-to-hack-the-hidden-system-config-repository)
4. [How to configure secrets used by Zuul](#how-to-configure-secrets-used-by-zuul)

## How to run helper services

You may need to spin up a Gerrit instance to host a **config** repository or other test repositories;
or you may need to run a Prometheus instance to develop monitoring rules for a deployment.

### Gerrit

You can deploy a test Gerrit with the CLI:

```sh
sf-operator dev create gerrit
```

## How to open a review on the test Gerrit

The checkout of the **config** repository is done by the [`dev create demo-env` command](./../../reference/cli/index.md#create-demo-env). Then, for example:

```sh
cd deploy/config
touch myfile
git add myfile && git commit -m"Add myfile"
git review
```

The **config-check** job is started and Zuul reports the build's results on the change.

As the **admin** user on Gerrit, the change can be approved with "CR +2, W+1", and then Zuul starts
the **config-check** job in the **gate** pipeline and the **config-update** job in
the **post** pipeline.

## How to hack the hidden system-config repository

The `system-config` repository is a hidden repository managed entirely by the SF-Operator. It defines
Zuul's default configuration: its default pipelines, jobs (notably the **config-update** and
**config-check** jobs), secrets and shared roles.

To hack on this configuration, you need to clone the **system-config** repository:

```sh
kubectl port-forward service/git-server-rw 9419
git clone git://localhost:9419/system-config /tmp/system-config
```

Make your changes, commit them, then push them with `git push`.

To test your modifications, you can simply create a trivial change on the **config** repository, as described [here](#how-to-open-a-review-on-the-test-gerrit).

## How to configure secrets used by Zuul

This Python package provides helper code to perform service runtime configuration.

Run locally: `tox -evenv -- sf-operator --help`
