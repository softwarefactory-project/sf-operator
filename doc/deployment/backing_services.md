# Backing services

Here you will find information about the backing services used by Zuul and Nodepool.

## Table of Contents

1. Philosophy
1. MariaDB
1. ZooKeeper

## Philosophy

The goal of SF-Operator is to provide a Zuul-based CI infrastructure on OpenShift with the least
friction possible. With that in mind, we decided to minimize the prerequisites to deployment, and
thus decided to integrate "barebones" deployments of the backing services required by Zuul and Nodepool.

The pros are:

* A deployer does not need to worry about provisioning a database or setting up a Zookeeper cluster
prior to deploying SF.

The cons are:

* Lifecycle support for these backing services is minimal (deployment, updates) compared to what a
proper operator-backed deployment could offer (see for example [what the mariadb-operator can do](https://mariadb.org/mariadb-in-kubernetes-with-mariadb-operator/)).

In other words, for backing services deployed as statefulsets, it is always possible to modify the replicas amount directly in their manifests, but SF-Operator will not act upon it - for example increasing mariadb's replicas will not set up a primary node and replica nodes like a dedicated mariadb operator would. You will only end up with one node being used by Zuul, and the rest using up resources for nothing.

## MariaDB

MariaDB provides a backend for Zuul's builds and buildsets results. It can be queried via zuul-web.

MariaDB is deployed as a single-pod statefulset.

## ZooKeeper

ZooKeeper coordinates data and configurations between all the Zuul and Nodepool microservices.

ZooKeeper is deployed as a single-pod statefulset.
