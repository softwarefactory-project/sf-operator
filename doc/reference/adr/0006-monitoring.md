---
status: proposed
date: 2023-06-16
---

# Operator and Operand Metrics Collection

## Context and Problem Statement

The Operator framework enables metrics publishing from the operator. The operands we manage also expose some metrics. We want to expose, collect and monitor
these metrics in a way that is consistent across deployment platforms ("vanilla" OpenShift and MicroShift), and abides by good and current practices in the matter.

## Considered Options

* Assume an external Prometheus, ie. the metrics must be exposed publicly.
* Assume the presence of the Prometheus operator shipped as a core component of OpenShift, and use user-defined projects monitoring
* Assume a user-deployed instance of Prometheus on any namespace with the capacity to grab metrics from the sf namespace.

## Decision Outcome

Assume user-defined projects monitoring on OpenShift.

### Consequences

* Good, because this is the expected way to handle metrics in an OpenShift cluster.
* Bad, because we may not be able (or it might be hard) to reproduce it in MicroShift, ie. dev and CI environments ... but "manual" deployment may be good enough in these cases.

## Pros and Cons of the Options

### External Prometheus

An external instance of Prometheus will be used to gather metrics. This requires the metrics to be pullable from that instance.

* Good, because we don't have much to do except expose the metrics endpoints with ingress rules.
* Bad, from a user standpoint, because the external Prometheus won't benefit from operator automation such as automated collection and alerting, especially in the cases of operands scaling up or down.

### OpenShift's user-defined projects monitoring

In OpenShift the Prometheus operator is a core component. Cluster monitoring is enabled by default, and "user space" monitoring
can be optionally enabled. User-defined apps' and operators' metrics can be automatically collected by the cluster-wide instance
dedicated to this type of monitoring. Users then have access to the Prometheus instance where RBAC ensures metrics isolation.

* Good, because we can automate metrics collection and alerting with our own defined ServiceMonitor and AlertRule CRs, as parts of the sf-operator.
* Good, because this is the intended way to manage metrics in users' projects.
* Good, because this ~should~ be compatible with an external Prometheus (either in federation mode or by using Thanos for metrics forwarding)
* Bad, because this monitoring stack is entirely optional and might be disabled on an OpenShift cluster.
* Bad, because it might be hard to set up MicroShift with the exact same monitoring stack as OpenShift, so we may not be aware of potential issues in dev mode.
  * It may be good enough to deploy a user-space instance of Prometheus via the manually installed Prometheus operator to simulate it.

### User-deployed instance of Prometheus

We have already experimented as a PoC with installing the Prometheus Operator on MicroShift + auto collecting metrics with a ServiceMonitor CR, see https://softwarefactory-project.io/r/c/software-factory/sf-operator/+/28632/1/

* Good, because we can provide documentation and a CLI utility to deploy a Prometheus instance, eventually even include it in our stack as an optional dependency.
* Good, because we can automate metrics collection and alerting with our own defined ServiceMonitor and AlertRule CRs, as parts of the sf-operator.
* Good, because we already know it works on MicroShift.
* Good, because it can work with an external Prometheus under federation.
* Bad, because this is not recommended on OpenShift, as it can interfere with and mess up the cluster monitoring stack. Users might not even be allowed to deploy Prometheus CRs.

## More Information

* Prometheus federation: https://prometheus.io/docs/prometheus/latest/federation/
* OpenShift documentation about user-defined projects monitoring https://docs.openshift.com/container-platform/4.13/monitoring/enabling-monitoring-for-user-defined-projects.html