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
   MY_NS=$(kubectl config view | awk '/namespace/ { print $2 }')
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
make dev-deployment
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
kubectl delete softwarefactory my-sf && kubectl apply -f config/samples && go run ./main.go --namespace $MY_NS
```

* Delete all pods, services etc

```sh
MY_NS=$(kubectl config view | awk '/namespace/ { print $2 }')
kubectl -n $MY_NS delete all  --all --now
for resource in certificates ClusterIssuers issuers certificaterequests secrets pvc configmaps deployments pods services ingress;
do
  kubectl -n $MY_NS delete $resource --all;
done
```


# sf_operator configuration library

This python package provides helpers code to perform service runtime configuration.

Run locally: `tox -evenv -- sf_operator --help`
