# Development and testing with MicroShift

This document provides instructions on how to deploy a [MicroShift](https://github.com/openshift/microshift) instance on a CentOS 9 Stream host, from your development computer.

We use Microshift as the target **OpenShift instance** for SF-Operator when deploying, developing locally, or testing in our [CI](https://microshift.softwarefactory-project.io/zuul/t/sf/builds?project=software-factory%2Fsf-operator&skip=0).

The deployment will be performed via sf-operator CLI.

1. [Requirements](#requirements)
1. [Install MicroShift](#install-microshift)

## Requirements

### Host

Here are the minimal and recommended specs for a MicroShift host:

| Property | Minimum | Recommended |
|------------|-------------|----------|
| CPUs/vCPUS | 2 | 4 |
| RAM | 8GB | 16GB |
| HDD | 40GB | 100GB |
| OS | CentOS Stream 9 | CentOS Stream 9 |

You must also be able to reach the following ports on the MicroShift host:

* TCP/6443
* TCP/22 (SSH)

Once the host is set up, ensure that your development machine can access
the Virtual Machine via SSH as a non-root user. Note that the non-root user should have
sudo privileges; usually this can be done by running the following command as root:

```sh
usermod -aG sudo <user>
```

You should also make sure your system is up-to-date and reboot after any kernel upgrades, by running

```sh
sudo dnf update -y
sudo shutdown -r now
```

You will also need the Host's public IP, or at least an IP you can reach from your development machine.

We will use a dedicated FQDN to set up the cluster; in this documentation the FQDN will be `microshift.dev`. Adapt the installation steps if you intend to use a different FQDN.

### Pull Secret

MicroShift requires a **pull secret** to access its container registry.

You can generate a pull secret [here](https://cloud.redhat.com/openshift/create/local) and download it to your development machine.

### Development computer

First, ensure you can resolve the **microshift.dev** FQDN to by adding an entry in your `/etc/hosts` file if necessary.

Then, install the following dependencies, for RPM-based systems:

```sh
sudo dnf install -y ansible-core golang
```

## Install MicroShift

You have to generate and adapt the configuration file to deploy microshift

```sh
go run main.go init config --dev > myconfig.yaml
```

Then edit `myconfig.yaml` as follow:

```yaml
contexts:
  my-context:
    development:
      microshift:
        disk-file-size: "30G"
        etcd-on-ramdisk: true
        host: "microshift.dev"
        openshift-pull-secret: '<paste-pull-secret-here>'
        user: "cloud-user"
default-context: my-context
```

Refer to the [`dev create microshift --config myconfig.yaml` subcommand](./../reference/cli/index.md#create-microshift) for more information about the command.

Finally run the sf-operator CLI as follow:

```sh
go run main.go --config ./myconfig.yaml dev create microshift --skip-local-setup
```

Once the deployment has ended successfully, you are now ready to deploy and hack SF-Operator, congratulations!
