# SF-Operator: a Zuul-based CI infrastructure for OpenShift

<a href="https://zuul-ci.org" ><img src ="https://zuul-ci.org/gated.svg" /></a>
<a href="https://softwarefactory-project.io/zuul/t/local/buildsets?project=software-factory%2Fsf-operator&pipeline=post&skip=0" ><img src="https://softwarefactory-project.io/zuul/api/tenant/local/badge?project=software-factory/sf-operator&pipeline=post" /></a>
<a href="https://matrix.to/#/#softwarefactory-project:matrix.org"><img src="https://img.shields.io/badge/matrix-%23softwarefactory--project-8A2BE2" alt="Matrix Channel" /></a>
<a href="https://github.com/softwarefactory-project/sf-operator/tags" ><img src="https://img.shields.io/github/v/tag/softwarefactory-project/sf-operator" /></a>
<img src="https://img.shields.io/badge/project%20status-ALPHA-FF2060" alt="Testing only; use in production at your own risks" />

## About

SF-Operator is an Operator that simplifies the deployment and operation of Software Factory instances on the OpenShift Container Platform. An instance of Software Factory is composed of [Zuul](https://zuul-ci.org) and its dependencies ([NodePool](https://zuul-ci.org/docs/nodepool/latest/), [Zookeeper](https://zookeeper.apache.org/doc/r3.8.3/index.html), [MariaDB](https://mariadb.org/documentation/#entry-header), [Log Server](./deployment/logserver.md)).

It is the natural evolution of the [Software Factory project](https://softwarefactory-project.io): the 3.8.x release of Software Factory saw the containerization of every major service, but was still delivered as RPM packages, in the form of a custom CentOS 7 distribution.
SF-Operator builds upon this containerization effort to move from a distro-centric approach to a cloud-native deployment.
This is also an opportunity to focus on the leanest service suite needed to provide a working gated CI infrastructure; hence a scope reduced to Zuul and its dependencies only.

SF-Operator is built mostly in Go upon the [Operator Framework](https://operatorframework.io), with the aim of reaching the highest capability level that can be achieved, in order to bring a modern, scalable gated CI alternative to OpenShift users, with the least friction from operation as possible.

Furthermore, SF-Operator takes advantage of some of the specificities of OpenShift as a container orchestration platform:

* Improved ingress management with OpenShift's Route Custom Resources
* Integration with OLM for streamlined operator and operands' lifecycle management
* If [enabled in OpenShift](https://docs.openshift.com/container-platform/4.13/monitoring/enabling-monitoring-for-user-defined-projects.html#enabling-monitoring-for-user-defined-projects), SF-Operator comes with default monitoring and alerting configurations that can be used out of the box. The default alerting rules are honed from years of maintaining and running several large Zuul deployments at scale for [Fedora](https://fedora.softwarefactory-project.io/zuul/status), [Ansible](https://ansible.softwarefactory-project.io/zuul/status) and [RDO](https://review.rdoproject.org/zuul/status).
* If [enabled](https://docs.openshift.com/container-platform/4.13/logging/cluster-logging.html), OpenShift provides application logs aggregation with its logging subsystem out of the box.

Finally, we also provide a [Command Line Interface (CLI)](reference/cli/index.md) to simplify common tasks related to the operator, management of the operands, development and testing.

## Status

The current project status is: **Alpha - NOT PRODUCTION READY**

## Capability Levels

* Level 1 - Basic Install - **10/10**
    - Zuul Scheduler: ✅
    - Zuul Executor: ✅
    - Zuul Web: ✅
    - Zuul Merger: ✅
    - Nodepool Launcher: ✅
    - Nodepool Builder: ✅
    - Zookeeper: ✅
    - MariaDB: ✅
    - Log Server: ✅
    - Internal Config Repository, bootstrapped pipelines and default jobs: ✅
* Level 2 - Seamless upgrades - **2/2**
    - Operator: ✅
    - Operands: ✅
* Level 3 - Full Lifecycle - **1/5**
    - SF 3.8.x migration ❌
    - Backup: ❌
    - Restore: ❌
    - Rolling deployments: ❌
    - Reconfiguration: ✅
* Level 4 - Deep Insights - **1/3**
    - Operator metrics: ❌
    - Operand metrics: ✅
    - Alerts: ❌ (WIP)
* Level 5 - Auto pilot - **0/3**
    - Auto-scaling : ❌
    - Auto-healing: ❌
    - Auto-tuning: ❌

## Getting Started

* [Installing the Operator ](https://softwarefactory-project.github.io/sf-operator/operator/getting_started/)
* [Deploying Zuul and dependencies with SF-Operator](https://softwarefactory-project.github.io/sf-operator/deployment/getting_started)

## Documentation

* [Operator documentation](https://softwarefactory-project.github.io/sf-operator/operator/): for OpenShift cluster administrators, this documentation covers installing SF-Operator and managing the operator's lifecycle.
* [Deployment documentation](https://softwarefactory-project.github.io/sf-operator/deployment/): this documentation covers the essentials for people or teams who intend to deploy and manage Zuul and its dependencies through the SF-Operator.
* [Developer documentation](https://softwarefactory-project.github.io/sf-operator/developer/): this documentation describes how to set up a development and testing environment to develop the SF-Operator.
* [End User documentation](https://softwarefactory-project.github.io/sf-operator/user/): for users of a Software Factory instance. This documentation mostly describes the `Software Factory's config repository` usage (configuration-as-code).
* [CLI refererence](https://softwarefactory-project.github.io/sf-operator/reference/cli/)

## Getting Help

Should you have any questions or feedback concerning the SF-Operator, you can:

* [Join our Matrix channel](https://matrix.to/#/#softwarefactory-project:matrix.org)
* Send an email to [softwarefactory-dev@redhat.com](mailto:softwarefactory-dev@redhat.com)
* [File an issue](https://github.com/softwarefactory-project/sf-operator/issues/new) for bugs and feature suggestions

## Contributing

Refer to [CONTRIBUTING.md](https://github.com/softwarefactory-project/sf-operator/blob/master/CONTRIBUTING.md).

## Licence

Sf-operator is distributed under the [Apache License](https://www.apache.org/licenses/LICENSE-2.0.txt).
