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
