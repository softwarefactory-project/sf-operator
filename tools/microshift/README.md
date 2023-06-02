# Deploying a Microshift Test Environment for SF-Operator

This document provides instructions on how to deploy a [Microshift](https://github.com/openshift/microshift) instance on a CentOS 9 Stream host.

We use Microshift as the target **OpenShift instance** for SF-Operator when deploying, developing locally, or testing in our **CI**.

The deployment will be performed via Ansible, by using the
[ansible-microshift-role](https://github.com/openstack-k8s-operators/ansible-microshift-role).

## Requirements

### Setup the target Host (Microshift)

The dedicated Virtual Machine should be a CentOS Stream 9 virtual machine
(min 2 vCPUs, 2 GB Ram, 40 GB Disk) with the TCP/6443 and TCP/22 (SSH) ports exposed.

Once the Virtual Machine is set up, ensure that your development machine can access
the Virtual Machine via SSH as a non-root user. Note that the non-root user should have
sudo privileges; usually this can be done running the following command as root on the VM:

```sh
usermod -aG sudo <user>
```

You should also make sure your system is up-to-date, by running

```sh
sudo dnf update -y
sudo shutdown -r now
```

The last command will reboot the Host, in case a new kernel needs
to be loaded.

You will also need the Host's public IP, or at least an IP you can reach from your development machine.

### Get the Pull Request secret for Microshift

Generate a **pull secret** [here](https://cloud.redhat.com/openshift/create/local) and download it to your development machine.

### Development Machine

First, ensure to have the **microshift.dev** to resolve the microshift machine in */etc/hosts*.

Then, install the following dependencies, for RPM-based systems:

```sh
sudo dnf install -y ansible-core golang
```

## Install Microshift

Installing Microshift is straightforward with the [ansible-microshift-role](https://github.com/openstack-k8s-operators/ansible-microshift-role).
We provide a cli tool sfconfig, that prepares, downloads and runs the role
on the target host.

You have to create your own inventory based on ./tools/microshift/inventory.yaml to set:

- the *openshift_pull_secret*: copy the content from the *pull-secret* file previously downloaded
- the *ansible_user* and *ansible_host* (to enable ansible connection via ssh to the microshift host)

```sh
cp ./tools/microshift/inventory.yaml ./tools/microshift/my-inventory.yaml
# Edit my-inventory.yaml and set the variables as explained above
```

Then from sf-operator directory, run

```sh
./tools/sfconfig microshift -i ./tools/microshift/my-inventory.yaml
```

Use the *--skip-local-setup* option to skip the setup phase on you local (dev) machine.

Once the deployment has ended successfully, you can configure kubectl to
use Microshift:

```sh
# Add microshift config to watched configs
export KUBECONFIG=${PWD}/microshift-config:~/.kube/config:$KUBECONFIG
kubectl config use-context microshift
```

You are now ready to deploy and hack SF-Operator, congratulations!
