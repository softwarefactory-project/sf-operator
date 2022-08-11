# sf-operator

The sf-operator deploys the Software Factory services.

## Getting Started
> The operator will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

1. Install the Custom Resource Definition:

```sh
make install
```

2. Install the demo Custom Resource:

```sh
kubectl apply -f config/samples/
```

3. Starts the operator:

```sh
MY_NS=$(kubectl config view | awk '/namespace/ { print $2 }')
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

## Cheat Sheet

```
# Get a service logs (from the first container):
kubectl logs deployment/keycloak -f

# Join a service container (by getting the container name with it's label):
function getPodName { kubectl get pods -lrun=$1 -o  'jsonpath={.items[0].metadata.name}'; }
kubectl exec -it $(getPodName "keycloak") sh

# Reset deployment
kubectl delete softwarefactory my-sf && kubectl apply -f config/samples && go run ./main.go --namespace $MY_NS
```
