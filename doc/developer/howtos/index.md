# Developer HOWTOs

This document lists tips and methods to perform tasks that may be useful to a developer.

## Table of Contents

1. [How to run helper services](#how-to-run-helper-services)
2. [How to open a review on the test Gerrit](#how-to-open-a-review-on-the-test-gerrit)
3. [How to hack the hidden system-config repository](#how-to-hack-the-hidden-system-config-repository)
4. [How to configure secrets used by Zuul](#how-to-configure-secrets-used-by-zuul)

## How to run helper services

You may need to spin a Gerrit instance to host a **config** repository or other test repositories;
or you may need to run a Prometheus instance to develop monitoring rules for a deployment.

### Gerrit

You can deploy a test Gerrit with the CLI:

```sh
./tools/sfconfig gerrit --deploy
```

### Prometheus

You can deploy a test Prometheus with the CLI:

```sh
./tools/sfconfig prometheus
```

This Prometheus instance is configured to collect metrics from a deployed Software Factory resource automatically.

## How to open a review on the test Gerrit

The checkout of the **config** repository is done by the `dev prepare` command.

```sh
cd deploy/config
touch myfile
git add myfile && git commit -m"Add myfile"
git review
```

The **config-check** job is started and Zuul reports the build's results on the change.

As the **admin** user on Gerrit, the change can be approved with "CR +2, W+1" then Zuul starts
the **config-check** job in the **gate** pipeline and the **config-update** job in
the **post** pipeline.

## How to hack the hidden system-config repository

The `system-config` repository is a hidden repository entirely managed by the SF-Operator. It defines
Zuul's default configuration: its default pipelines, jobs (notably the **config-update** and
**config-check** jobs), secrets and shared roles.

To hack on this configuration, you need to clone the **system-config** repository:

```sh
kubectl port-forward service/git-server 9418
git clone git://localhost:9418/system-config /tmp/system-config
```

Make your changes, commit them, then push them with `git push`.

To test your modifications, you can simply create a trivial change on the **config** repository, as described [here](#how-to-open-a-review-on-the-test-gerrit).

## How to hack on the LogServer Custom Resource

The operator handles the `LogServer` Custom Resource. This resource is used to setup the logs server
part of a `SoftwareFactory` deployment.

Here is an usage example of this resource:

```shell
# Create a dedicated namespace
kubectl create ns logserver
# Start the operator for the dedicated namespace
go run ./main.go --namespace logserver operator
```

```shell
# Load your public ssh key in base64
PUB_KEY=`cat ~/.ssh/id_ecdsa.pub | base64 -w0`
# Create the resource manifest
sed "s/authorizedSSHKey.*/authorizedSSHKey: $PUB_KEY/" config/samples/sf_v1_logserver.yaml > /tmp/my-logserver.yaml
sed "s/fqdn.*/fqdn: test.local/" -i /tmp/my-logserver.yaml
# Apply the custom resource
kubectl apply -f /tmp/my-logserver.yaml
```

To access the web frontend of the service you need to ensure that `logserver.test.local` resolves to your
microshift cluster inbound, then `firefox https://logserver.test.local`.

To send data to the logserver, first enable the port-forward:

```shell
kubectl -n logserver port-forward service/logserver 22220:2222
```

Then use rsync:

```shell
rsync -av -e "ssh -p22220" src-directory zuul@127.0.0.1:rsync/
```

## How to configure secrets used by Zuul

This python package provides helper code to perform service runtime configuration.

Run locally: `tox -evenv -- sf_operator --help`