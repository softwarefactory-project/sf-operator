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
- python-kubernetes

Furthermore the `operator-sdk` is needed when you need to generate/update the OLM bundle or
when a new `CRD` needs to be added to the operator. Here is the installation process:

```
make operator-sdk
```

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

4. Apply the SoftwareFactory Custom Resource:

   ```sh
   kubectl apply -f ./my-sf.yaml
   ```

5. Start the operator:

   ```sh
   go run ./main.go --namespace sf
   ```

### Access services with the browser

If the FQDN is not already configured to point at your kubernetes cluster inbound,
then you need to setup a local entry in /etc/hosts.

Verify services by running:

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

However:

- **Persistent Volumes Claims** and not cleaned after the deletion of the softwarefactory instance
- `tools/run-ci-tests.sh` deploys resources via OLM into the `bundle-catalog-ns` namespace

To fully wipe such resources run the following command:

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
./tools/run-ci-tests.sh --test-only
```

The command accepts extra Ansible parameters. For instance to override
the default `microshift_host` var:

```sh
./tools/run-ci-tests.sh --extra-var "microshift_host=my-microshift"
```

To fetch the test suite artifacts locally, run:

```sh
./tools/fetch-artifacts.sh
```

Artifacts will be available in the `/tmp/sf-operator-artifacts/` directory.

### sfconfig cli tool

#### Adding a new option

Install cobra-cli

``` bash
go install github.com/spf13/cobra-cli@latest
```

``` bash
cd cli/sfconfig
~/go/bin/cobra-cli add myCommand
cd -
```

Then you can edit `cli/sfconfig/cmd/myCommand.go` to add the needed option

The option can be directly used after editing myCommand.go with

``` bash
go run cli/sfconfig/main.go myCommand
```

A wrapper also exists on /tools directory

``` bash
./tools/sfconfig myCommand
```

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

## Hack on the LogServer Custom Resource

The operator handles the `LogServer` Custom Resource. This resource is used to setup the logs server
part of a `SoftwareFactory` deployment.

Here is an usage example of this resource:

```shell
# Create a dedicated namespace
kubectl create ns logserver
# Start the operator for the dedicated namespace
go run ./main.go --namespace logserver
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

To access the web frontend of the service you need to ensure that `logserver.test.local` resolve to your
microshift cluster inbound, then `firefox https://logserver.test.local`.

To send data to the logserver, first enable the port-forward:

```shell
kubectl -n logserver port-forward service/logserver-sshd 22220:2222
```

Then use rsync:

```shell
rsync -av -e "ssh -p22220" src-directory zuul@127.0.0.1:rsync/
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
```

#### Create new PatchSet in the Gerrit service

Sometimes, when it would be necessary to debug the Gerrit and Zuul CI workflow,
there is a script that creates a PatchSet in the Gerrit and it would be later
processed by the Zuul CI.
To use it, run:

```sh
bash ./tools/create-ps.sh
```

#### Images management

##### Modify an existing image

If you want to modify an image for testing, you can use buildah after an initial deployment
on microshift instance as root (eg add acl package on zuul-executor). The example below adds the
acl package on the zuul-executor image:

```sh
[root@microshift ~]# dnf install -y buildah
[root@microshift ~]# CTX=$(buildah from quay.io/software-factory/zuul-executor:8.2.0-3)
[root@microshift ~]# buildah run $CTX microdnf install -y acl
[root@microshift ~]# buildah commit --rm $CTX quay.io/software-factory/zuul-executor:8.2.0-3
```

Then you can wipe the deployment and redeploy to use the newly built image.

##### Create an image from a Containerfile

If you want to build an image from a Containerfile, you can use buildah to create it on the microshift instance as root.
The example below creates a sf-op-busybox image

```sh
[root@microshift ~]# dnf install -y buildah
[root@microshift ~]# buildah bud -f Containerfile -t sf-op-busybox:1.4-4
```

For microshift deployment uses cri-o runtime, you can list images with:

```
[root@microshift ~]# crictl images | grep sf-op-busybox
localhost/sf-op-busybox                                       1.4-4               c9befa3e7ebf6       885MB
```

Then modify the controller go file where the image is defined (here it's controllers/utils.go file)
```go
const BUSYBOX_IMAGE = "localhost/sf-op-busybox:1.4-4"
```

Then you can do the deployment to use the newly built image.
