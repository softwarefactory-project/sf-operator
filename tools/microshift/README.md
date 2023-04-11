# Deploying a Microshift Test Environment for SF-Operator

This document provides instructions on how to deploy a [Microshift](https://github.com/openshift/microshift) instance on a CentOS 9 Stream host.

We use Microshift as the target **OpenShift instance** for SF-Operator when deploying, developing locally, or testing in our **CI**.

The deployment will be performed via Ansible, by using the
[ansible-microshift-role](https://github.com/openstack-k8s-operators/ansible-microshift-role).

## Requirements

### Target Host (Microshift)

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
sudo yum update -y
sudo shutdown -r now
```

The last command will reboot the Host, in case a new kernel needs
to be loaded.

You will also need the Host's public IP, or at least an IP you can reach from your development machine.

### Pull Secret

Generate a **pull secret** [here](https://cloud.redhat.com/openshift/create/local) and download it to your development machine. We will assume it was downloaded to `$HOME/Downloads/pull-secret`.

### Development Machine

Install the following dependencies, for RPM-based systems:

* ansible-core
* git
* kubectl

```sh
sudo dnf install -y ansible-core git kubectl
```

Once Ansible Core is installed, install the following collections:

* community.general
* community.crypto
* ansible.posix

```sh
ansible-galaxy collection install community.general community.crypto ansible.posix
```

## Install Microshift

Installing Microshift is straightforward with the [ansible-microshift-role](https://github.com/openstack-k8s-operators/ansible-microshift-role).
We provide a playbook, `deploy-microshift.yaml`, that downloads and runs the role
on the target host.

Set some environment variables:

```sh
# Replace with the IP address of your target Microshift instance
export MICROSHIFT_IP=1.2.3.4
# Setup host resolution
echo "${MICROSHIFT_IP} microshift.dev gerrit.sftests.com zuul.sftests.com logserver.sftests.com" | sudo tee -a /etc/hosts
# Replace with the actual path to your pull-secret
export PULL_SECRET=$(cat ${HOME}/Downloads/pull-secret)
# the user that can be used to ssh into the Microshift instance
export MICROSHIFT_USER=cloud-user
```

Make sure the `inventory.yaml` file defines the right user to SSH into the Microshift target.

Then run the deployment playbook:

```sh
ansible-playbook -i inventory.yaml deploy-microshift.yaml --extra-vars "openshift_pull_secret=${PULL_SECRET} microshift_user=${MICROSHIFT_USER}"
```

Once the playbook has ended successfully, you can configure kubectl to
use Microshift:

```sh
# Add microshift config to watched configs
export KUBECONFIG=${PWD}/microshift-config:~/.kube/config:$KUBECONFIG
kubectl config use-context microshift
```

You are now ready to deploy and hack SF-Operator, congratulations!