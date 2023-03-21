# sf-operator

The sf-operator deploys the Software Factory services.

Project status : Work In Progress

## ADR

Architecture Decision Records are available as Markdown format in *doc/adr/*.

To add a new decision:

1. Copy doc/adr/adr-template.md to doc/adr/NNNN-title-with-dashes.md, where NNNN indicates the next number in sequence.
2. Edit NNNN-title-with-dashes.md.

More information in the [ADR's README](doc/adr/README.md).

## System requirement

You need to install the following dependencies on your dev machine:
- make
- golang
- ansible-core

## Run the SF operator in devel mode

The operator will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).
Make sure that your current context is called `microshift` and use a namespace called `default`.

Be sure to use a dedicated k8s dev instance. We are using `microshift` for that purpose.

```sh
kubectl config current-context
# Must be microshift
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

4. Ensure Storage Class "topolvm-provisioner" available:

   The operator expects a Storage Class named "topolvm-provisioner"

   ```sh
   kubectl get sc
   ```

   The default SC name can be overriden by using "storageClassName" CRD's field if needed.

5. Starts the operator:

   ```sh
   go run ./main.go --namespace default --cr "./my-sf.yaml"
   ```

## Access services with the browser

If the FQDN is not already configured to point at your kubernetes cluster inbound,
then you need to setup a local entry in /etc/hosts:

```sh
echo "${MICROSHIFT_IP} zuul.sftests.com gerrit.sftests.com | sudo tee -a /etc/hosts
firefox https://zuul.sftests.com
firefox https://gerrit.sftests.com
```

Or

```sh
curl -k https://${MICROSHIFT_IP} -H "HOST: gerrit.sftests.com"
curl -k https://${MICROSHIFT_IP} -H "HOST: zuul.sftests.com"
```

## Reset a deployment

```sh
kubectl delete softwarefactory my-sf
go run ./main.go --namespace default --cr "./my-sf.yaml"
```

## Run ci test locally

You can run CI tests on your local microshift deployment with

```sh
./tools/run_ci_tests.sh
```

This command is a wrapper on top of `ansible-playbook` to run the same Ansible play
than the CI. The command accepts extra Ansible parameters. For instance to override
the default `microshift _host` var:

```sh
./tools/run_ci_tests.sh --extra-vars "microshift_host=my-microshift"
```

We run the same playbook used by zuul during CI jobs

## Wipe all content in dev namespace

Deleting the SoftwareFactory resource keeps persistent volume and some secrets. To
wipe all in your namespace, runs:

Note that this also delete all Persistent Volumes on the cluster.

```sh
./tools/wipe-deployment.sh
```

## sf_operator configuration library

This python package provides helpers code to perform service runtime configuration.

Run locally: `tox -evenv -- sf_operator --help`

## Interact with the deployment

### Open a review on the internal Gerrit - from your host

First checkout the **config** repository.

To do so via HTTP:

```sh
# Get the Gerrit admin user API key
./tools/get-secret.sh gerrit-admin-api-key
# Then checkout the config repository
git clone "https://admin@gerrit.sftests.com/a/config" /tmp/config
cd /tmp/config
git remote add gerrit "https://admin:<gerrit-admin-api-key>@gerrit.sftests.com/a/config"
```

To do so via SSH:

```sh
# In a terminal
kubectl port-forward service/gerrit-sshd 29418
# In another terminal
git clone ssh://admin@localhost:29418/config /tmp/config
cd /tmp/config
git config user.email admin@sftests.com
sed -i "s/^host=.*/host=localhost/" .gitreview
git review -s # Enter 'admin' user
git checkout .gitreview
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
