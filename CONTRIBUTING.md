# Contributing

This document provides instructions to get started with **sf-operator** development.

## Code Hosting

The main repository of the project is hosted at [softwarefactory-project.io][https://softwarefactory-project.io/r/software-factory/sf-operator).

Container images definitions are hosted at the [container-pipeline project](https://softwarefactory-project.io/r/containers) and
published on [quay.io](https://quay.io/organization/software-factory).

Any contributions are welcome. Use the [git-review workflow](https://softwarefactory-project.io/docs/user/contribute.html#create-a-new-code-review) to interact with these projects.

Repositories on GitHub are mirrors from **softwarefactory-project.io**.

## Requirements

We use [Microshift](https://github.com/openshift/microshift) for the **CI** and **developer OpenShift instances**.

### Microshift deployment

We recommend to install Microshift on a dedicated Virtual Machine using the
[ansible-microshift-role](https://github.com/openstack-k8s-operators/ansible-microshift-role).
The dedicated Virtual Machine should be a CentOS Stream 9 virtual machine
(min 2 vCPUs, 2 GB Ram, 20 GB Disk) with the TCP/6443 and TCP/22 (SSH) ports exposed.

Currently a *Pull Secret* is needed. It can be generated [here](https://cloud.redhat.com/openshift/create/local).

Once the Virtual Machine is set up, ensure that your development machine can access via SSH as a regular user
the Virtual Machine. Furthermore ensure that the regular user can access root user with `sudo -i`.

On you development machine:

```sh
# Replace with the IP address of your Microshift instance
export MICROSHIFT_IP=10.42.0.2

# Setup the server IP
echo "${MICROSHIFT_IP} microshift.dev gerrit.sftests.com zuul.sftests.com logserver.sftests.com" | sudo tee -a /etc/hosts

# Install confmgmt tools
sudo dnf install -y ansible-core git
ansible-galaxy collection install community.general community.crypto ansible.posix

# Setup recipe
mkdir -p deploy; cd deploy
git clone https://github.com/openstack-k8s-operators/ansible-microshift-role
cat << EOF > ansible.cfg
[defaults]
roles_path = ./
force_handlers = True
EOF

cat << EOF > deploy-microshift.yaml
- hosts: microshift.dev
  vars:
    fqdn: microshift.dev
    disk_file_sparsed: true
    standard_user: false
    create_pv: false
    openshift_pull_secret: |
      <COPY THE PULL SECRET HERE>
  roles:
    - ansible-microshift-role
  tasks:
    # This overwrite your local kube/config so backup it before and comment that task.
    - fetch:
        src: /var/lib/microshift/resources/kubeadmin/microshift.dev/kubeconfig
        dest: "~/.kube/config"
        flat: true
      become: yes
EOF

cat << EOF > inventory.yaml
[microshift.dev]
microshift.dev:
  # Adapt the username if needed
  ansible_user: cloud-user
EOF

# Run recipe
ansible-playbook -i inventory.yaml deploy-microshift.yaml

# Validate deployment
kubectl cluster-info
```

### Dependencies for hacking on the sf-operator

You need to install the following dependencies on your dev machine:

- kubectl
- git
- golang # >= 1.19
- make
- ansible-core
- jq
- python-tox

## Hack on SF operator

### Run the operator in devel mode

The operator will automatically use the current context in your kubeconfig file
(i.e. whatever cluster `kubectl cluster-info` shows). Make sure that your current context is
called `microshift` and use a namespace called `default`.

```sh
kubectl config current-context
# Must be microshift
```

1. Install cert-manager operator

   ```sh
   # Ensure cert-manager operator is installed on your instance
   make install-cert-manager
   ```

2. Install the Custom Resource Definition:

   ```sh
   make install
   ```

3. Create your own copy the CR sample, for example:

   ```sh
   cp config/samples/sf_v1_softwarefactory.yaml my-sf.yaml
   ```

4. Start the operator:

   ```sh
   go run ./main.go --namespace default --cr "./my-sf.yaml"
   ```

### Access services with the browser

If the FQDN is not already configured to point at your kubernetes cluster inbound,
then you need to setup a local entry in /etc/hosts:

```sh
firefox https://zuul.sftests.com
firefox https://gerrit.sftests.com
firefox https://logserver.sftests.com/logs
```

### Delete a deployment

This removes the `my-sf` Custom Resource instance.

```sh
kubectl delete softwarefactory my-sf
```

However **secrets**, **Persistent Volumes Claims** and **Dynamically provisonned Persistent Volumes**
are kept and not deleted automatically. To fully wipe a deployment run:

```sh
./tools/wipe-deployment.sh
```

### Run test suites locally

Tests run by the **CI** (`playbooks/main.yaml`) can be also run locally using the `run-ci-tests.sh`:

```sh
./tools/run-ci-tests.sh
```

This command is a wrapper on top of `ansible-playbook` to run the same Ansible play
than the CI. This includes the operator deployment and testing.

If you want to only run the testing part (the functional tests only, assuming the operator
deployed a Software Factory instance), you can use the `test_only` tag:

```sh
./tools/run-ci-tests.sh --tags test_only
```

The command accepts extra Ansible parameters. For instance to override
the default `microshift_host` var:

```sh
./tools/run-ci-tests.sh --extra-vars "microshift_host=my-microshift"
```

### Interact with the deployment

#### Open a review on the internal Gerrit - from your host

First checkout the **config** repository.

```sh
# Get the Gerrit admin user API key
gerrit_admin_api_key=$(./tools/get-secret.sh gerrit-admin-api-key)
# Then checkout the config repository
git -c http.sslVerify=false clone "https://admin:${gerrit_admin_api_key}@gerrit.sftests.com/a/config" /tmp/config
cd /tmp/config
git config http.sslverify false
git remote add gerrit "https://admin:${gerrit_admin_api_key}@gerrit.sftests.com/a/config"
```

Then add a change and send the review:

```sh
touch myfile
git add myfile && git commit -m"Add myfile"
git review
```

The **config-check** job is started and Zuul votes on change.

As the **admin** user on Gerrit, the change can be approved with "CR +2, W+1" then Zuul starts
the **config-check** job in the **gate** pipeline and the **config-update** job in
the **post** pipeline.

#### Config Update Job

To modify or debug the **config-update** job you need a copy of the **system-config** repository:

```sh
kubectl port-forward service/git-server 9418
git clone git://localhost:9418/system-config /tmp/system-config
```

After changing the playbooks or tasks, just `git push`.

Finally, trigger a new `config-update` by running the following command:

```sh
( cd /tmp/config &&
  date > trigger &&
  git add trigger && git commit -m "Trigger job" && git review && sleep 1 && git push gerrit
)
```

## sf_operator configuration library

This python package provides helper code to perform service runtime configuration.

Run locally: `tox -evenv -- sf_operator --help`

## Debugging Tips

### Troubleshooting

### OC Debug

A good way to debug pods is using the [oc debug](https://docs.openshift.com/container-platform/4.8/cli_reference/openshift_cli/developer-cli-commands.html#oc-debug) command.

The debug command makes an exact copy of the container passed as argument. It even has the option to select the user to start with.

#### Examples
```
oc debug <container to copy>
oc debug <container to copy> --as-root
oc debug <container to copy> --as-user=<username>
```
