# sf-operator

The sf-operator deploys the Software Factory services.

Project status : Work In Progress

## ADR

Architecture Decision Records are available as Markdown format in *doc/adr/*.

To add a new decision:

1. Copy doc/adr/adr-template.md to doc/adr/NNNN-title-with-dashes.md, where NNNN indicates the next number in sequence.
2. Edit NNNN-title-with-dashes.md.

More information in the [ADR's README](doc/adr/README.md).

## Run the SF operator in devel mode

The operator will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows). Make sure that your developer context is called `microshift`.

Be sure to use a dedicated k8s dev instance. We are using `microshift` for that purpose.

```sh
kubectl config current-context
# Must be microshift
```

0. Install cert-manager operator

   ```sh
   # Ensure cert-manager operator is installed on your instance
   make install-cert-manager
   ```

1. Install the Custom Resource Definition:

   ```sh
   make install
   ```

2. Set user namespace as variable:

   ```sh
   # Then run the command by setting up the current context name
   MY_NS=$(kubectl config view -o jsonpath='{.contexts[?(@.name == "microshift")].context.namespace}')
   echo $MY_NS
   ```

3. Replace the Custom Resource information, for example:

   ```sh
   # Copy the Custom Resoure sample
   cp config/samples/sf_v1_softwarefactory.yaml my-sf.yaml
   # change FQDN
   export FQDN="${MY_NS}.sftests.com"
   sed -i "s/fqdn: \"sftests.com\"/fqdn: \"${FQDN}\"/g" my-sf.yaml
   ```

4. Starts the operator:

   ```sh
   go run ./main.go --namespace $MY_NS --cr "./my-sf.yaml"
   ```

## Access services with the browser

If the FQDN is not already configured to point at your kubernetes cluster inbound,
then you need to setup a local entry in /etc/hosts:

```sh
echo "${K8S_EXTERNAL_IP} ${FQDN} zuul.${FQDN} gerrit.${FQDN} | sudo tee -a /etc/hosts
firefox http://zuul.${FQDN}/
```

## Reset a deployment

```sh
kubectl delete softwarefactory my-sf
go run ./main.go --namespace $MY_NS --cr "./my-sf.yaml"
```

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

Add your local SSH key to the gerrit demo user. "demo" user password is "demo". Then:

```sh
kubectl port-forward service/gerrit-sshd 29418
cd /tmp
git clone ssh://demo@localhost:29418/config
git config user.email demo@fbo-sftests.com
sed -i "s/^host=.*/host=localhost/" .gitreview
git review -s
git checkout .gitreview
```

Now any change can be done and push via the "git review" command.

### Open a review on the internal Gerrit - from the gerrit pod

We will use the gerrit admin account.

```sh
kubectl exec -it gerrit-0 bash
pip3 install --user git-review
cd /tmp
git clone ssh://gerrit/config
cd config
git review -s
```

Now any change can be done and push via the "git review" command.

Use the following command to approve a change:

```sh
ssh gerrit gerrit review 1,1 --code-review "+2" --workflow "+1"
```
