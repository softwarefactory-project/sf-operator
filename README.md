<img src ="https://zuul-ci.org/gated.svg" />
<img src="https://softwarefactory-project.io/zuul/api/tenant/local/badge?project=software-factory/sf-operator&pipeline=post" />
<a href="https://matrix.to/#/#softwarefactory-project:matrix.org"><img src="https://img.shields.io/badge/matrix-%23softwarefactory--project-8A2BE2" alt="Matrix Channel" /></a>
<img src="https://img.shields.io/github/v/tag/softwarefactory-project/sf-operator" />
<img src="https://img.shields.io/badge/project%20status-ALPHA-FF2060" alt="Testing only; use in production at your own risks" />

# SF-Operator: a Zuul-based CI infrastructure for OpenShift

## About

SF-Operator is an Operator that simplifies the deployment and operation of [Zuul](https://zuul-ci.org) and its dependencies (NodePool, Zookeeper, MariaDB, Log Server) on the OpenShift Container Platform.

It is the natural evolution of the [Software Factory project](https://softwarefactory-project.io): the 3.8.x release of Software Factory saw the containerization of every major service, but was still delivered as RPM packages, in the form of a custom CentOS 7 distribution.
SF-Operator builds upon this containerization effort to move from a distro-centric approach to a cloud-native deployment.
This is also an opportunity to focus on the leanest service suite needed to provide a working gated CI infrastructure; hence a scope reduced to Zuul and its dependencies only.

SF-Operator is built mostly in Go upon the [Operator Framework](https://operatorframework.io), with the aim of reaching the highest capability level that can be achieved, in order to bring a modern, scalable gated CI alternative to OpenShift users, with the least friction from operation as possible. 

Furthermore, SF-Operator takes advantage of some of the specificities of OpenShift as a container orchestration platform:

* Improved routes API
* Integration with OLM for streamlined operator and operands' lifecycle management
* If [enabled in OpenShift](https://docs.openshift.com/container-platform/4.13/monitoring/enabling-monitoring-for-user-defined-projects.html#enabling-monitoring-for-user-defined-projects), SF-Operator comes with default monitoring and alerting configurations that can be used out of the box. The default alerting rules are honed from years of maintaining and running several large Zuul deployments at scale for [Fedora](https://fedora.softwarefactory-project.io/zuul/status), [Ansible](https://ansible.softwarefactory-project.io/zuul/status) and [RDO](https://review.rdoproject.org/zuul/status).
* If [enabled](https://docs.openshift.com/container-platform/4.13/logging/cluster-logging.html), OpenShift provides application logs aggregation with its logging subsystem out of the box.

Finally, we also provide a Command Line Interface (CLI) called sfconfig to simplify common tasks related to the operator, management of the operands, development and testing.

## Status

The current project status is: **Alpha - NOT PRODUCTION READY**

## Capability Levels

* Level 1 - Basic Install - **8/10**
    - Zuul Scheduler: ✅
    - Zuul Executor: ✅
    - Zuul Web: ✅
    - Zuul Merger: ❌
    - Nodepool Launcher: ✅
    - Nodepool Builder: ❌
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

* [Installing the Operator ](doc/operator/getting_started.md)
* [Deploying Zuul and dependencies with SF-Operator](doc/deployment/getting_started.md)

## Documentation

* [Operator documentation](doc/operator/index.md): for OpenShift cluster administrators, this documentation covers installing SF-Operator and managing the operator's lifecycle.
* [Deployment documentation](doc/deployment/index.md): this documentation covers the essentials for people or teams who intend to deploy and manage Zuul and its dependencies through the SF-Operator.
* [Developer documentation](doc/developer/index.md): this documentation describes how to set up a development and testing environment to develop the SF-Operator.
* [CLI refererence](doc/cli/index.md)
## Getting Help

Should you have any questions or feedback concerning the SF-Operator, you can:

* [Join our Matrix channel](https://matrix.to/#/#softwarefactory-project:matrix.org)
* Send an email to [softwarefactory-dev@redhat.com](softwarefactory-dev@redhat.com)
* [File an issue](https://github.com/softwarefactory-project/sf-operator/issues/new) for bugs and feature suggestions

## Contributing

Refer to [CONTRIBUTING.md](CONTRIBUTING.md).

## Licence

Sf-operator is distributed under the [Apache License](LICENSE).
