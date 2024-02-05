---
status: proposed
date: 2023-05-08
---

# Command line tool to setup an manage sf-operator deployment

## Context and Problem Statement

This ADR discuss what tooling could be used to setup and manage operations with sf-operator

With last sf release with rpm (3.8), sfconfig was used to:

* deploy an sf deployment
* upgrade an sf deployment
* update (manage) an sf deployment:
** add a gerrit connection with /etc/software-factory/sfconfig.yaml
** add new instances for a service (like zuul-executor or nodepool-launcher) with /etc/software-factory/arch.yaml
** override defaults /etc/software-factory/custom-vars.yaml
** Get the status of the deployment (with testinfra) at the end of the sfconfig cmd
* recover (backup was done by ansible-playbook with cron)
* erase an sf deployment
* provision a demo project

As an operator of sf-4 with sf-operator needs to have tooling to:

* setup microshift if needed or use existing k8s deployment
* install the tooling and operator:
** install microshift (assume centos stream 9)
** perform the deployment of sf-operator via OLM
** reclaim a SF in a given namespace
* have tooling to create, update, delete, and test the operator
* fetch access to the deployment (gerrit ssh key)
* generate a CRD template for operators to manually edit or fill
* get the status of the deployment
* perform a backup
* restore a backup

## Considered Options

### shell scripts

Collection of shell scripts on /tools directory and ansible playbooks to help to setup k8s deployment and run ci test

### single binary (sfconfig)

* a single binary written in go (like the operator) to manage and interact with the operator or the deployment
* the binary should be `part of the sf-operator binary`

## Decision Outcome

A single binary

## Pros and Cons of the Options

### Pros

* easier to maintain than a bunch of shell scripts
* functional tests and unit testing is simplier with a single binary
* can be tested on the ci like the previous sfconfig python tool
* cli tool written with cobra [1] could be directly used without rebuild (eg `go run cli/sfconfig/main.go action`)
* ansible-microshift role can be used to setup microshift with go-ansible lib

[1] https://github.com/spf13/cobra
[2] https://github.com/apenella/go-ansible

### Cons
