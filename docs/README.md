# SF-Operator: a Zuul-based CI infrastructure for OpenShift

<a href="https://zuul-ci.org" ><img src ="https://zuul-ci.org/gated.svg" /></a>
<a href="https://softwarefactory-project.io/zuul/t/local/buildsets?project=software-factory%2Fsf-operator&pipeline=post&skip=0" ><img src="https://softwarefactory-project.io/zuul/api/tenant/local/badge?project=software-factory/sf-operator&pipeline=post" /></a>
<a href="https://matrix.to/#/#softwarefactory-project:matrix.org"><img src="https://img.shields.io/badge/matrix-%23softwarefactory--project-8A2BE2" alt="Matrix Channel" /></a>
<a href="https://github.com/softwarefactory-project/sf-operator/tags" ><img src="https://img.shields.io/github/v/tag/softwarefactory-project/sf-operator" /></a>
<img src="https://img.shields.io/badge/project%20status-BETA-orange" alt="This project is in beta phase. Use in production at your own risks." />

SF-Operator is a Kubernetes deployment system to install Software Factory (SF) on the OpenShift Container Platform.
SF is a Continuous Integration (CI) service based on [Zuul](https://zuul-ci.org) to provide project gating with Ansible
for developer platforms like GitLab using cloud providers like OpenStack.

## Getting Started

To try SF-Operator you will need:

- A Linux system to run the operator standalone (RHEL, CentOS or Fedora are supported).
- A copy of the source code.
- Access to a Kubernetes or OpenShift cluster (recommended).

Run the following commands:

```ShellSession
git clone https://softwarefactory-project.io/r/software-factory/sf-operator
cd sf-operator
./hack/deploy.sh
```

> This procedure is expected to work out of the box on a fresh system, please create a [bug report][bugreport] if it ever fails.

You now have the Software Factory services running at the default `https://sfop.me` domain.

The local Zuul is connected to a Gerrit instance running at `https://gerrit.sfop.me`. You can test the integration by creating a change in the `demo-project` and watch Zuul run a job.

First, you need to install the `git-review` plugin:

```ShellSession
sudo dnf install -y git-review
```

Then, create a test change:

```ShellSession
cd deploy/demo-project
git checkout -b demo
echo "Hello world" > README.md
git add README.md
git commit -m "My first review"
git review
```

The change will be submitted to the Gerrit server. You can then visit the Zuul status page at `https://sfop.me` to see the `demo-job` running for your change.

The next steps are:

- Configure the custom resource (CR) to change the FQDN, add Zuul connections and Nodepool providers.
- Configure public access to the SF services using a Route or Ingress.
- Create a Zuul tenant in the provided project config.
- Manage the project config through the config-update pipelines by hosting the configuration on your developer platform.

The next sections introduce the SF architecture and the installation options.

## Architecture Overview

Software Factory is composed of the following services:

- Zuul, for running the CI.
- Gateway, for the HTTP frontend.
- Nodepool, for the resource providers.
- LogServer, for the build logs.
- HoundSearch, for code search.
- LogJuicer, for build analysis.
- Weeder, for inspecting whole tenant config.

Internally, SF leverages the following services:

- ZooKeeper, for the Zuul state.
- MariaDB, for storing the build results.
- GitServer, for hosting the internal tenant config.
- Gerrit, for the initial config project location until it is moved to an external location.

SF-Operator deployment is defined with the following elements:

- The Software Factory custom resource (CR) to manage the services configuration.
- The project config repository to manage the Zuul tenants and Nodepool providers.

## Install

SF-Operator supports OpenShift and Kubernetes, and it is tested with the following local cluster deployment:

- MicroShift for OpenShift
- Minikube for Kubernetes

Besides the Getting Started process, here are the available installation modes:

- Run the SF-Operator in standalone mode (recommended).
- Install the CRD and deploy the SF-Operator on the cluster to manage the SF resources with kubectl.

For example, here is a standard configuration resource (CR), adapted from the getting started one, which is available in [sf.yaml](https://github.com/softwarefactory-project/sf-operator/blob/master/playbooks/files/sf.yaml):

```yaml
apiVersion: sf.softwarefactory-project.io/v1
kind: SoftwareFactory
metadata:
  name: my-sf
spec:
  fqdn: "sfop.me"
  config-location:
    name: sf/project-config
    zuul-connection-name: my-gitlab
  zuul:
    gitlabconns:
      - name: my-gitlab
        server: gitlab.me
        baseurl: https://gitlab.me
        secrets: gitlab-secret
```

Which can be installed by running:

```ShellSession
# Make sure the ~/.kube/config is valid
oc login --token=SECRET --server=https://openshift.me:6443

# Apply the CR
go run ./main.go deploy ./my-sf.yaml
```

Check out the other documentation:

* [Deploying Zuul and dependencies with SF-Operator](deployment/getting_started.md)
* [Developing the SF-Operator](developer/getting_started.md)

## About

SF-Operator is the natural evolution of the [Software Factory project](https://softwarefactory-project.io): the 3.8.x release of Software Factory saw the containerization of every major service, but was still delivered as RPM packages, in the form of a custom CentOS 7 distribution.
SF-Operator builds upon this containerization effort to move from a distro-centric approach to a cloud-native deployment.
This is also an opportunity to focus on the leanest service suite needed to provide a working gated CI infrastructure; hence a scope reduced to Zuul and its dependencies only.

SF-Operator is built mostly in Go upon the [Operator Framework](https://operatorframework.io), with the aim of reaching the highest capability level that can be achieved, in order to bring a modern, scalable gated CI alternative to OpenShift users, with the least friction from operation as possible.

Furthermore, SF-Operator takes advantage of some of the specificities of OpenShift as a container orchestration platform:

* Improved ingress management with OpenShift's Route Custom Resources
* If [enabled in OpenShift](https://docs.openshift.com/container-platform/4.13/monitoring/enabling-monitoring-for-user-defined-projects.html#enabling-monitoring-for-user-defined-projects), SF-Operator comes with default monitoring and alerting configurations that can be used out of the box. The default alerting rules are honed from years of maintaining and running several large Zuul deployments at scale for [Fedora](https://fedora.softwarefactory-project.io/zuul/status), [Ansible](https://ansible.softwarefactory-project.io/zuul/status) and [RDO](https://review.rdoproject.org/zuul/status).
* If [enabled](https://docs.openshift.com/container-platform/4.13/logging/cluster-logging.html), OpenShift provides application logs aggregation with its logging subsystem out of the box.

Finally, we also provide a [Command Line Interface (CLI)](reference/cli/index.md) to simplify common tasks related to the operator, management of the operands, development and testing.

## Status

The current project status is: **Beta**

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
* Level 3 - Full Lifecycle - **3/5**
    - SF 3.8.x migration ❌
    - Backup: ✅
    - Restore: ✅
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

* [Deploying Zuul and dependencies with SF-Operator](deployment/getting_started.md)

## Documentation

* [Deployment documentation](deployment/index.md): this documentation covers the essentials for people or teams who intend to deploy and manage Zuul and its dependencies through the SF-Operator.
* [Developer documentation](developer/index.md): this documentation describes how to set up a development and testing environment to develop the SF-Operator.
* [End User documentation](user/index.md): for users of a Software Factory instance. This documentation mostly describes the `Software Factory's config repository` usage (configuration-as-code).
* [CLI reference](reference/cli/index.md)

## Getting Help

Should you have any questions or feedback concerning the SF-Operator, you can:

* [Join our Matrix channel](https://matrix.to/#/#softwarefactory-project:matrix.org)
* Send an email to [softwarefactory-dev@redhat.com](mailto:softwarefactory-dev@redhat.com)
* [File an issue][bugreport] for bugs and feature suggestions

## Contributing

Refer to [CONTRIBUTING.md](developer/CONTRIBUTING.md).

## License

Sf-operator is distributed under the [Apache License](https://www.apache.org/licenses/LICENSE-2.0.txt).

[bugreport]: https://github.com/softwarefactory-project/sf-operator/issues/new
