# Backing services

Here you will find information about the backing services used by Zuul and Nodepool.


1. [Philosophy](#philosophy)
1. [MariaDB](#mariadb)
1. [ZooKeeper](#zookeeper)

## Philosophy

The goal of SF-Operator is to provide a Zuul-based CI infrastructure on OpenShift with as little
friction as possible. With that in mind, we decided to minimize the prerequisites to deployment, and
thus decided to integrate "barebones" deployments of the backing services required by Zuul and Nodepool.

The pros are:

* A deployer does not need to worry about provisioning a database or setting up a ZooKeeper cluster
prior to deploying SF.

The cons are:

* Lifecycle support for these backing services is minimal (deployment, updates) compared to what a
proper operator-backed deployment could offer (see for example [what the mariadb-operator can do](https://mariadb.org/mariadb-in-kubernetes-with-mariadb-operator/)).

In other words, for backing services deployed as statefulsets, it is always possible to modify the replicas amount directly in their manifests, **but SF-Operator will not act upon it** - for example increasing mariadb's replicas will not set up a primary node and replica nodes like a dedicated mariadb operator would. You will only end up with one node being used by Zuul, and the rest using up resources for nothing.

Generally speaking, the backing services are best left untouched and managed by the SF Operator.

## MariaDB

MariaDB provides a backend for Zuul's builds and buildsets results. It can be queried via zuul-web.

MariaDB is deployed as a single-pod statefulset.

## ZooKeeper

ZooKeeper coordinates data and configurations between all the Zuul and Nodepool microservices.

ZooKeeper is deployed as a single-pod statefulset; The SF operator enforces this replica count, meaning that if the statefulset was edited to run 0 or more than one replica
the operator will scale it up or down to one replica.

### Certificates

Zuul and Nodepool services authenticate to ZooKeeper using an X509 client certificate. `sf-operator` manages a local Certificate Infrastructure (self-signed Certificate Authority, server and clients certificates). Those certificates are set with a long validity period (30 years) and an operator might want to rotate those certificates for security reasons. To do so:

Delete `secret` resources named:

- zookeeper-server-tls
- zookeeper-client-tls
- ca-cert

Roll out the following `Statefulset` and `Deployment` resources:

- zookeeper
- zuul-scheduler
- zuul-merger
- zuul-executor
- zuul-web
- weeder
- nodepool-builder
- nodepool-launcher

Then make sure to trigger the `Reconcile` loop of the `sf-operator` by running the `standalone` command.

### Debugging

Since zookeeper's client port is TLS-protected, a wrapper for ZkClient.sh was added inside the container to set the proper env variables enabling access to the secure port.
The wrapper can be found as `/tmp/zkCli`.