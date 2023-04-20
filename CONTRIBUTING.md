# Contributing

This document provides instructions to get started with **sf-operator** development.

## Code Hosting

The main repository of the project is hosted at [softwarefactory-project.io](https://softwarefactory-project.io/r/software-factory/sf-operator).

Container images definitions are hosted at the [container-pipeline project](https://softwarefactory-project.io/r/containers) and
published on [quay.io](https://quay.io/organization/software-factory).

All contributions are welcome. Use the [git-review workflow](https://softwarefactory-project.io/docs/user/contribute.html#create-a-new-code-review) to interact with these projects.

Repositories on GitHub are mirrors from **softwarefactory-project.io**.

## Requirements

We use [Microshift](https://github.com/openshift/microshift) as the target **OpenShift instance** for SF-Operator when deploying, developing locally, or testing in our **CI**.

We provide [instructions and a deployment script](./tools/microshift/README.md) in the `tools/microshift` folder if you need help setting up your own instance.

### Dependencies for hacking on the sf-operator

You need to install the following dependencies on your dev machine:

- kubectl
- git
- golang # >= 1.19
- make
- ansible-core
- jq
- python-tox
- git-review

## Hack on SF operator

### Run the operator in devel mode

The operator will automatically use the current context in your kubeconfig file
(i.e. whatever cluster `kubectl cluster-info` shows).
Make sure that your current context is called `microshift`.

```sh
kubectl config current-context
# Must be microshift
```

0. Create a new namespace

   ```sh
   kubectl create namespace sf
   kubectl config set-context microshift --namespace=sf
   kubectl label --overwrite ns sf pod-security.kubernetes.io/enforce=privileged
   kubectl label --overwrite ns sf pod-security.kubernetes.io/enforce-version=v1.24
   oc adm policy add-scc-to-user privileged -z default
   ```

1. Install cert-manager operator

   ```sh
   # Ensure cert-manager operator is installed on your instance
   make install-cert-manager
   ```

2. Install the Custom Resource Definition:

   ```sh
   make install
   ```

3. Create your own copy the CR sample, for example:

   ```sh
   cp config/samples/sf_v1_softwarefactory.yaml my-sf.yaml
   ```

4. Start the operator:

   ```sh
   go run ./main.go --namespace sf --cr ./my-sf.yaml
   ```

### Access services with the browser

If the FQDN is not already configured to point at your kubernetes cluster inbound,
then you need to setup a local entry in /etc/hosts:

```sh
firefox https://zuul.sftests.com
firefox https://gerrit.sftests.com
firefox https://logserver.sftests.com/logs
```

### Delete a deployment

This removes the `my-sf` Custom Resource instance.

```sh
kubectl delete softwarefactory my-sf
```

However **secrets**, **Persistent Volumes Claims** and **Dynamically provisonned Persistent Volumes**
are kept and not deleted automatically. To fully wipe a deployment run:

```sh
./tools/wipe-deployment.sh
```

### Run test suites locally

Tests run by the **CI** (`playbooks/main.yaml`) can be also run locally using the `run-ci-tests.sh`:

```sh
./tools/run-ci-tests.sh
```

This command is a wrapper on top of `ansible-playbook` to run the same Ansible play
than the CI. This includes the operator deployment and testing.

If you want to only run the testing part (the functional tests only, assuming the operator
deployed a Software Factory instance), you can use the `test_only` tag:

```sh
./tools/run-ci-tests.sh --tags test_only
```

The command accepts extra Ansible parameters. For instance to override
the default `microshift_host` var:

```sh
./tools/run-ci-tests.sh --extra-vars "microshift_host=my-microshift"
```

To fetch the test suite artifacts locally, run:

```sh
./tools/fetch-artifacts.sh
```

Artifacts will be available in the `/tmp/sf-operator-artifacts/` directory.

### Interact with the deployment

#### Open a review on the internal Gerrit - from your host

First checkout the **config** repository.

```sh
# Get the Gerrit admin user API key
gerrit_admin_api_key=$(./tools/get-secret.sh gerrit-admin-api-key)
# Then checkout the config repository
git -c http.sslVerify=false clone "https://admin:${gerrit_admin_api_key}@gerrit.sftests.com/a/config" /tmp/config
cd /tmp/config
git config http.sslverify false
git remote add gerrit "https://admin:${gerrit_admin_api_key}@gerrit.sftests.com/a/config"
```

Then add a change and send the review:

```sh
touch myfile
git add myfile && git commit -m"Add myfile"
git review
```

The **config-check** job is started and Zuul votes on change.

As the **admin** user on Gerrit, the change can be approved with "CR +2, W+1" then Zuul starts
the **config-check** job in the **gate** pipeline and the **config-update** job in
the **post** pipeline.

#### Config Update Job

To modify or debug the **config-update** job you need a copy of the **system-config** repository:

```sh
kubectl port-forward service/git-server 9418
git clone git://localhost:9418/system-config /tmp/system-config
```

After changing the playbooks or tasks, just `git push`.

Finally, trigger a new `config-update` by running the following command:

```sh
( cd /tmp/config &&
  date > trigger &&
  git add trigger && git commit -m "Trigger job" && git review && sleep 1 && git push gerrit
)
```

## sf_operator configuration library

This python package provides helper code to perform service runtime configuration.

Run locally: `tox -evenv -- sf_operator --help`

## Debugging Tips

### Troubleshooting

### OC Debug

A good way to debug pods is using the [oc debug](https://docs.openshift.com/container-platform/4.8/cli_reference/openshift_cli/developer-cli-commands.html#oc-debug) command.

The debug command makes an exact copy of the container passed as argument. It even has the option to select the user to start with.

#### Examples
```
oc debug <container to copy>
oc debug <container to copy> --as-root
oc debug <container to copy> --as-user=<username>
```

#### Checking service name resolution

Normally, if the service is [headless](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services),
all containers in the cluster should be able to resolve the service ip address,
or even resolve pod ip address, related to that service.
For example:

```sh
kubectl exec -it mariadb-0 -- bash -c "host zookeeper-0.zookeeper-headless.default.svc.cluster.local"
kubectl exec -it mariadb-0 -- bash -c "host zuul-executor-0.zuul-executor-headless.default.svc.cluster.local"
kubectl exec -it mariadb-0 -- bash -c "host zuul-executor-0.zuul-executor-headless"

#### Create new PatchSet in the Gerrit service

Sometimes, when it would be necessary to debug the Gerrit and Zuul CI workflow,
there is a script that creates a PatchSet in the Gerrit and it would be later
processed by the Zuul CI.
To use it, run:

```sh
bash ./tools/create-ps.sh
```
