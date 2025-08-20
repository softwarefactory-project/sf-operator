# Troubleshooting

This document lists some methods that can be used to debug the various components of SF-Operator.


1. [OC Debugging](#oc-debugging)
1. [go-delve](#go-delve)
1. [Checking service name resolution](#checking-service-name-resolution)

## OC Debugging

A good way to debug pods is to use the [oc debug](https://docs.openshift.com/container-platform/4.13/cli_reference/openshift_cli/developer-cli-commands.html#oc-debug) command.

The debug command makes an exact copy of the pod passed as an argument. It will then start a shell session as any specified user.

### Examples
```
oc debug <container to copy>
oc debug <container to copy> --as-root
oc debug <container to copy> --as-user=<username>
```

## go-delve

Delve is a Go debugger. Follow this [documentation](https://github.com/go-delve/delve/tree/master/Documentation/installation) to install it.

### Example debugging session

Let's assume an error is occurring in the function `DeployLogserver`, and to be more precise,
when the function `UpdateR` is called within `DeployLogServer`.

To check calls and analyze code:

```sh
# in that step, we assume that an SF resource has been created beforehand, and we are doing a re-run
# of the sf-operator in dev mode.
dlv debug main.go --namespace sf
```

With delve running, we can add breakpoints like this:

```sh
# this will add a breakpoint in line 272
break controllers/logserver_controller.go:272

# if you want to analyze the whole function, you can do:
break DeployLogserver
```

Then after setting the breakpoint, we can just add:

```sh
continue
```

Short cheat sheet:

```sh
# shows local variables
locals

# show what the "annotations" variable contains
print annotations

# show current step/code
list (alias l)

# overwrite a variable
set annotations = <something>

# call a function, e.g.:
call r.Client.Status().Update(ctx, &cr)
```

Please read delve's [documentation](https://github.com/go-delve/delve/blob/master/Documentation/cli/getting_started.md) for further details. 

## Checking service name resolution

Normally, if the service is [headless](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services),
all containers in the cluster should be able to resolve the service's IP address,
or even resolve the service's pod IP address.
You can check that name resolution is working properly by running these commands:

```sh
kubectl exec -it mariadb-0 -- bash -c "host zookeeper-0.zookeeper-headless.default.svc.cluster.local"
kubectl exec -it mariadb-0 -- bash -c "host zuul-executor-0.zuul-executor-headless.default.svc.cluster.local"
kubectl exec -it mariadb-0 -- bash -c "host zuul-executor-0.zuul-executor-headless"
```