# Hacking images

This document explains how to modify or interact with images and services used by the SF-Operator
for development purposes.

!!! note
    These instructions assume you are using a MicroShift deployment for development.


1. [Root access inside containers](#root-access-inside-containers)
1. [Modify an existing image](#modify-an-existing-image)
1. [Create and use an image from a Containerfile](#create-and-use-an-image-from-a-containerfile)
1. [Edit Zuul source code and mount in a pod](#edit-the-zuul-source-code-and-mount-in-a-pod)

## Root access inside containers

!!! danger
    These instructions should only be followed for development purposes, and may end up breaking your deployment. Use at your own risks!

1. Edit the target deployment, statefulset or pod directly. For example, with `nodepool-launcher`:

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

Then you can wipe the deployment and redeploy to use the newly built image.


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
mkdir -p /home/centos/git && cd /home/centos/git
git clone https://opendev.org/zuul/zuul
# Clone at the current version provided by sf-operator or use master branch
git checkout 9.3.0
```

Then you can run the operator by providing the environment variable `ZUUL_LOCAL_SOURCE=<full-path-to-zuul-source>`.
For instance (using the standalone mode):

```
ZUUL_LOCAL_SOURCE=/home/cloud-user/git/zuul/zuul sf-operator --namespace sf dev create standalone-sf --cr playbooks/files/sf.yaml
```

After any code change, you can restart the Zuul pods, for instance the zuul-scheduler pod:

```
kubectl rollout restart -n sf sts/zuul-scheduler
kubectl rollout restart -n sf sts/zuul-executor
kubectl rollout restart -n sf sts/zuul-merger
kubectl rollout restart -n sf deploy/zuul-web
```

### Zuul-web

zuul-web static assets are located under the Zuul source tree on the container image. The local copy, from
the git clone, does not provide the static assets.

Either,

- [Build the static assets](https://zuul-ci.org/docs/zuul/latest/developer/javascript.html#deploying) and store them into
/usr/local/lib/python3.11/site-packages/zuul/web/static.
- Fetch the built asset from the zuul-web container image (see below).

To Fetch the built asset from the zuul-web container image, run the following process from the microshift machine.

```sh
cd /home/centos/git/zuul/zuul/web
podman create --name zuul-web quay.io/software-factory/zuul-web:9.3.0-1
podman export -o /tmp/zuul-web.tar zuul-web
tar -xf /tmp/zuul-web.tar usr/local/lib/python3.11/site-packages/zuul/web/static
mv usr/local/lib/python3.11/site-packages/zuul/web/static static
rm -Rf usr
```

