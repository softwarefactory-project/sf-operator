# Nodepool

Here you will find information about managing the Nodepool service when deployed with the SF Operator.
It does not replace [Nodepool's documentation](https://zuul-ci.org/docs/nodepool/latest/),
but addresses specificities and idiosyncrasies induced by deploying Nodepool with the SF Operator.

## Table of Contents

1. [Architecture](#architecture)
1. [Services configuration](#services-configuration)
1. [Setting up providers secrets](#setting-up-providers-secrets)
1. [Using a cloud image in an OpenStack cloud](#using-a-cloud-image-in-an-openstack-cloud)
1. [Using the openshifpods driver with your cluster](#using-the-openshiftpods-driver-with-your-cluster)
1. [Using the Nodepool CLI](#using-the-nodepool-cli)

## Architecture

Nodepool is deployed by SF-Operator as micro-services:

| Service | Kubernetes resource type | Scalable Y/N |
|---------|--------------------------|-------------|
| nodepool-launcher | deployment | N |

The operator also includes backing services with bare bones support:

| Service | Kubernetes resource type | Scalable Y/N |
|---------|--------------------------|-------------|
| zookeeper | statefulset | N |

> Although Zookeeper is deployed as a statefulset, modifying its replica count directly in its manifest
will have no effect on the service itself - besides eventually creating unused pods.
## Services configuration

Configuring the Nodepool micro-services is done through the SoftwareFactory deployment's manifest. Many configuration parameters are exposed by The [SoftwareFactory Custom Resource spec](./../../config/crd/bases/sf.softwarefactory-project.io_softwarefactories.yaml).

The spec is constantly evolving during alpha development, and should be considered
unstable but the ultimate source of truth for documentation about its properties.

## Setting up provider secrets

Currently the SF Operator supports OpenStack (`clouds.yaml`) and Kubernetes (`kube.config`) configuration files. These files are used by Nodepool to manage resources on its providers.
They are managed by the SF Operator in a secret called `nodepool-providers-secrets`.

To push your configuration file(s) to Nodepool:

1. Edit your [sfconfig.yaml](./../../sfconfig.yaml) to add the paths to your configuration files:

```yaml
ansible_microshift_role_path: ~/src/github.com/openstack-k8s-operators/ansible-microshift-role
microshift:
  host: microshift.dev
  user: cloud-user
fqdn: sftests.com
nodepool:
  clouds_file: /path/to/clouds.yaml
  kube_file: /path/to/kube/config
```

2. Run sfconfig:

```sh
./tools/sfconfig nodepool-providers-secrets --upload
```

3. Wait until your deployment becomes ready again:

```sh
kubectl get sf -n sf -w
```

When your deployment is ready, the provider secrets have been updated in Nodepool.

> You can also fetch the currently used configurations by running `./tools/sfconfig nodepool-providers-secrets --dump` ,
which will copy the configuration files from Nodepool into the files defined in sfconfig.yaml. Be careful not to erase
important data!

## Using a cloud image in an OpenStack cloud

There is a simple way to provision a cloud image with Zuul's SSH key, so that Zuul can run jobs on images in an OpenStack cloud.

1. If not done already, clone the [config repository for your deployment](./config_repository.md).
2. Edit `nodepool/nodepool.yaml` to add labels and providers:

```yaml
labels:
-   name: my-cloud-image-label
    # min-ready: 1
providers:
- name: default
  cloud: default
  clean-floating-ips: true
  image-name-format: '{image_name}-{timestamp}'
  boot-timeout: 120 # default 60
  cloud-images:
    - name: my-cloud-image
      username: cloud-user
  pools:
    - name: main
      max-servers: 10
      networks:
        - $public_network_name
      labels:
        - cloud-image: cloud-centos-9-stream
          name: cloud-centos-9-stream
          flavor-name: $flavor
          userdata: |
            #cloud-config
            package_update: true
            users:
              - name: cloud-user
                ssh_authorized_keys:
                  - $zuul-ssh-key
```

3. Save, commit, propose a review and merge the change.
4. Wait for the **config-update** job to complete.
5. If the `min-ready` property is over 0, you can validate that an instance is available with:

```sh
$ kubectl exec -ti nodepool-launcher-$uuid -c launcher -- nodepool list
```

An instance with your new image label should appear in the list. You can take note of its public IP for the next step.

6. You can also make sure a Zuul Executor pod can connect to the instance:

```sh
$kubectl exec -ti zuul-executor-0 -- ssh -o "StrictHostKeyChecking no" -i /var/lib/zuul-ssh/..data/priv cloud-user@$public_ip hostname
Warning: Permanently added '$public_ip' (ED25519) to the list of known hosts.
np0000000001
```

## Using the Openshiftpods driver with your cluster

Nodepool's [Openshiftpods driver](https://zuul-ci.org/docs/nodepool/latest/openshift-pods.html) enables
Zuul to request pods from an OpenShift cluster to run jobs.

We recommend using a dedicated namespace with the driver (if you intend to spawn pods in the same OpenShift cluster than the one your deployment lives in).

The `sfconfig` CLI can automate the creation of such a namespace, and set up the associated kube config as a Nodepool secret:

```sh
sfconfig create-namespace-for-nodepool --nodepool-context my-context --nodepool-namespace nodepool-pods
```

Once Nodepool is ready, add the following snippet in the `nodepool/nodepool.yaml` file in your `config` repository:

```yaml
providers:
  - name: openshift-pods
    driver: openshiftpods
    context: my-context
    pools:
      - name: main
        labels:
          - name: my-pod
            image: quay.io/fedora/fedora:38
```

Commit your change, review it and validate it. After a run of `config-update`, your new provider and
labels will be available in Nodepool.

## Using the Nodepool CLI

The `nodepool` command line utility is available on `nodepool-launcher` pods.

To get the list of currently running launcher pods:

```sh
kubectl get pods --selector run=nodepool-launcher
```

Open a shell on any running `nodepool-launcher` pod listed by the previous command:

```sh
kubectl exec --stdin --tty nodepool-launcher-XYZ -- /bin/sh
```

Then from that shell, run the `nodepool` command.
