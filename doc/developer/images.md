# Hacking images

This document explains how to modify or interact with images and services used by the SF-Operator
for development purposes.

!!! note
    These instructions assume you are using a MicroShift deployment for development.


1. [Root access inside containers](#root-access-inside-containers)
1. [Modify an existing image](#modify-an-existing-image)
1. [Create and use an image from a Containerfile](#create-and-use-an-image-from-a-containerfile)
1. [Edit Zuul source code and mount in a pod](#edit-the-zuul-source-code-and-mount-in-a-pod)
1. [Upgrading an image](#upgrading-an-image)

## Root access inside containers

!!! danger
    These instructions should only be followed for development purposes, and may end up breaking your deployment. Use at your own risk!

1. Edit the target deployment, statefulset, or pod directly. For example, with `nodepool-launcher`:

```sh
kubectl edit deployment.apps/nodepool-launcher
```

2. Find and edit the following sections:

```yaml
    spec:
      ...
      containers:
        securityContext:
          privileged: true
      ...
      securityContext: {}
```

3. Save your changes and wait for the affected pod to be recreated.
4. You can now run `kubectl exec` to get a root shell.


## Modify an existing image

If you want to modify an image for testing, you can use [buildah](https://buildah.io/).
It requires that at least one SoftwareFactory Custom Resource has been created on your MicroShift
instance (to populate the local registry).
The example below adds the `acl` package on the zuul-executor image; as root on the MicroShift instance, run:

```sh
[root@microshift ~]# CTX=$(buildah from quay.io/software-factory/zuul-executor:8.2.0-3)
[root@microshift ~]# buildah run $CTX microdnf install -y acl
[root@microshift ~]# buildah commit --rm $CTX quay.io/software-factory/zuul-executor:8.2.0-3
```

Then you can wipe the deployment and redeploy it to use the newly built image.


## Create and use an image from a Containerfile

If you want to build an image from a Containerfile, you can use [buildah](https://buildah.io/) to create it on your MicroShift instance as root.

In this example, let's create a custom `sf-op-busybox` image to use with the operator. Assuming you defined the container in `ContainerFile`, and as root on the MicroShift instance, run:

```sh
[root@microshift ~]# buildah build -f Containerfile -t sf-op-busybox:1.4-4
```

On a MicroShift deployment using the CRI-O runtime, you can list images with:

```sh
[root@microshift ~]# crictl images | grep sf-op-busybox
localhost/sf-op-busybox                                       1.4-4               c9befa3e7ebf6       885MB
```

Then modify the file where the image is defined (here it's in the `controllers/utils.go` file)

```go
const BUSYBOX_IMAGE = "localhost/sf-op-busybox:1.4-4"
```

Then you can re-do a deployment to use the newly built image.

## Edit the Zuul source code and mount in a pod

We provide a facility (currently only for Zuul) to mount a local source tree on the
running Zuul pods.

Follow these steps on your MicroShift instance

```sh
mkdir -p /home/cloud-user/git && cd /home/cloud-user/git
git clone https://opendev.org/zuul/zuul
# Clone at the current version provided by sf-operator or use master branch
cd zuul; git checkout 10.1.0
```

Then you can run the operator by providing the environment variable `ZUUL_LOCAL_SOURCE=<full-path-to-zuul-source>`.
For instance (using the standalone mode):

```
ZUUL_LOCAL_SOURCE=/home/cloud-user/git/zuul/zuul go run main.go --namespace sf deploy playbooks/files/sf.yaml
```

After any code change, you can restart the Zuul pods, for instance the zuul-scheduler pod:

```
kubectl rollout restart -n sf sts/zuul-scheduler
kubectl rollout restart -n sf sts/zuul-executor
kubectl rollout restart -n sf sts/zuul-merger
kubectl rollout restart -n sf deploy/zuul-web
```

The Zuul web UI will not work; to re-enable it, see the [following section](#zuul-web).

### Zuul-web

zuul-web static assets are located under the Zuul source tree in the container image. The local copy from
the git clone does not provide the static assets.

Either,

- [Build the static assets](https://zuul-ci.org/docs/zuul/latest/developer/javascript.html#deploying) and store them into
/usr/local/lib/python3.11/site-packages/zuul/web/static.
- Fetch the built assets from the zuul-web container image (see below).

To fetch the built assets from the zuul-web container image, run the following process from the MicroShift machine.

```sh
cd /home/cloud-user/git/zuul/zuul/web
podman create --name zuul-web quay.io/software-factory/zuul-web:10.1.0-1
podman export -o /tmp/zuul-web.tar zuul-web
tar -xf /tmp/zuul-web.tar usr/local/lib/python3.11/site-packages/zuul/web/static
mv usr/local/lib/python3.11/site-packages/zuul/web/static static
rm -Rf usr
```

### Upgrading an image

This paragraph describes the container image upgrade process, whether a major component version, a security fix, or a custom patch is released.

This project's containers are managed in the [containers](https://softwarefactory-project.io/r/plugins/gitiles/containers) repository.
The first step is to clone this repository locally.

The containerfiles and versions are managed with Dhall, in the `images-sf/master` directory of the repository.

#### General process

1. Submit your change to the `containers` repo.
1. When this change is merged, edit the `controllers/libs/base/static/images.yaml` in the sf-operator repo to reflect
   the new version and container ID.
1. Submit your change.

#### Upgrading Zuul and Nodepool

Upgrading Zuul and Nodepool requires a few extra steps:

##### Containers Repo

1. Ensure python dependencies are up to date by running `make update-pip-freeze`

##### SF-Operator Repo

*Prerequisite*: you need a local clone of [zuul](https://opendev.org/zuul/zuul) and [nodepool](https://opendev.org/zuul/nodepool).

**Zuul:**

1. Set your local Zuul repo to the desired tag:

```sh
cd <path/to/zuul> && git fetch --all && git checkout <tag>
```

2. Run the statsd mapper utility tool in the `sf-operator` repo:

```sh
cd <path/to/sf-operator>
python hack/zuuldoc2statsdmapper.py -i <path/to/zuul>/doc/source/monitoring.rst controllers/static/zuul/statsd_mapping.yaml
```

3. Validate that the generated configuration can run with statsd-exporter (make sure the container version matches the one in
   `controllers/libs/base/static/images.yaml`)

```sh
podman run --rm -v ./controllers/static/zuul/statsd_mapping.yaml:/tmp/statsd_mapping.yaml:z docker.io/prom/statsd-exporter:v0.27.1 --statsd.mapping-config=/tmp/statsd_mapping.yaml
```

Any issue with the configuration will appear in the application logs. Otherwise, if all goes well, the last log should be similar to
`level=info msg="Accepting Prometheus Requests" addr=:9102`.

**Nodepool:**

1. Set your local Nodepool repo to the desired tag:

```sh
cd <path/to/nodepool> && git fetch --all && git checkout <tag>
```

2. Run the statsd mapper utility tool in the `sf-operator` repo:

```sh
cd <path/to/sf-operator>
python hack/zuuldoc2statsdmapper.py -i <path/to/nodepool>/doc/source/operation.rst controllers/static/nodepool/statsd_mapping.yaml.tmpl
```

3. Validate that the generated configuration can run with statsd-exporter (make sure the container version matches the one in
   `controllers/libs/base/static/images.yaml`)

```sh
podman run --rm -v ./controllers/static/nodepool/statsd_mapping.yaml.tmpl:/tmp/statsd_mapping.yaml:z docker.io/prom/statsd-exporter:v0.27.1 --statsd.mapping-config=/tmp/statsd_mapping.yaml
```

Any issue with the configuration will appear in the application logs. Otherwise, if all goes well, the last log should be similar to
`level=info msg="Accepting Prometheus Requests" addr=:9102`.

4. Uncomment the templated part at the end of `controllers/static/nodepool/statsd_mapping.yaml.tmpl`.

Finally, check the diffs on `controllers/static/nodepool/statsd_mapping.yaml.tmpl` and `controllers/static/zuul/statsd_mapping.yaml`
and document any changes (new or removed metrics) in the CHANGELOG.
