# sf-operator

The sf-operator deploys the Software Factory services.

## Getting Started

> The operator will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

1. Install the Custom Resource Definition:

   ```sh
   make install
   ```

2. Set user namespace as variable:

   ```sh
   MY_NS=$(kubectl config view -o jsonpath='{.contexts[].context.namespace}')
   ```

3. Replace the Custom Resource information, for example:

   ```sh
   # change FQDN
   sed -i "s/fqdn: \"sftests.com\"/fqdn: \"${MY_NS}.sftests.com\"/g" config/samples/sf_v1_softwarefactory.yaml

   # Enable all available services
   sed -i 's/false/true/g' config/samples/sf_v1_softwarefactory.yaml
   ```

4. Install the demo Custom Resource:

   ```sh
   kubectl apply -f config/samples/
   ```

5. Starts the operator:

   ```sh
   go run ./main.go --namespace $MY_NS
   ```

## Configure deployment DNS

You can validate the ingress is working using cURL:

```sh
curl http://${K8S_EXTERNAL_IP}/ -H "HOST: ${FQDN}"
```

If the FQDN is not already configured to point at your kubernetes cluster inbound,
then you need to setup a local entry in /etc/hosts:

```sh
echo "${K8S_EXTERNAL_IP} ${FQDN} zuul.${FQDN} gerrit.${FQDN} opensearch.${FQDN}" | sudo tee -a /etc/hosts
```

## Development

Above steps are also included in Makefile.
It might be interesting to run on development.

```sh
MY_NS=$(kubectl config view -o jsonpath='{.contexts[].context.namespace}')
if [ -z "${MY_NS}" ]; then
    MY_NS=default
fi
kubectl create namespace $MY_NS
make install
make dev-deployment
```

## Development CRC

The CRC deployment requires `pull-secret.txt` file, which can be downloaded
from [here](https://cloud.redhat.com/openshift/create/local).
How to deploy CRC you can find in `extra/crc` ansible role located in
sf-infra project.

To recreate sf-operator environment on holded node:

* run first steps defined in `Delete all content related to the sf-operator`
* create pv storage by executing:

```sh
cd ~/install_yamls/
make crc_storage
```

* Re-run sf-operator

```sh
cd /home/zuul-worker/src/softwarefactory-project.io/software-factory/sf-operator
kubectl delete softwarefactory my-sf
kubectl apply -f config/samples
go run ./main.go --namespace $MY_NS
```

It is also possible to pass the custom resources directly to the operator

```sh
go run ./main.go --namespace $MY_NS --oneshot --cr ~/my-sf.yaml
```

## Cheat Sheet

* Get a service logs (from the first container)

```sh
kubectl logs deployment/keycloak -f
``````

* Join a service container (by getting the container name with it's label)

```sh
function getPodName { kubectl get pods -lrun=$1 -o  'jsonpath={.items[0].metadata.name}'; }
kubectl exec -it $(getPodName "keycloak") sh
```

* Reset deployment

```sh
kubectl delete softwarefactory my-sf
kubectl apply -f config/samples
go run ./main.go --namespace $MY_NS
```

### Delete all content related to the sf-operator

Deleting the SoftwareFactory resource keeps persistent volume and some secrets. To
wipe all in your namespace, runs:

```sh
./tools/wipe-deployment.sh
```

# sf_operator configuration library

This python package provides helpers code to perform service runtime configuration.

Run locally: `tox -evenv -- sf_operator --help`

## ADR

Architecture Decision Records are available as Markdown format in *doc/adr/*.

To add a new decision:

1. Copy doc/adr/adr-template.md to doc/adr/NNNN-title-with-dashes.md, where NNNN indicates the next number in sequence.
2. Edit NNNN-title-with-dashes.md.

More information in the [ADR's README](doc/adr/README.md).
