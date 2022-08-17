# sf-operator

The sf-operator deploys the Software Factory services.

## Getting Started
> The operator will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

1. Install the Custom Resource Definition:

```sh
make install
```

2. Edit the demo Custom Resource to your liking:

```sh
cp ./config/samples/sf_v1_softwarefactory.yaml sf.yaml
$EDITOR sf.yaml
```

3. Deploy the Custom Resource:

```sh
MY_NS=$(kubectl config view | awk '/namespace/ { print $2 }')
go run ./main.go --namespace $MY_NS --cr ./sf.yaml
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

# Delete all pods, services etc.
kubectl -n dpawlik delete all  --all --now
for resource in certificates ClusterIssuers issuers certificaterequests secrets pvc configmaps deployments pods services; do kubectl delete $resource --all;done
```
