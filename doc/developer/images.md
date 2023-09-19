# Hacking images

This document explains how to modify or interact with images and services used by the SF-Operator
for development purposes.

> These instructions assume you are using a MicroShift deployment for development.

## Table of Contents

1. [Root access inside containers](#root-access-inside-containers)
1. [Modify an existing image](#modify-an-existing-image)
1. [Create and use an image from a Containerfile](#create-and-use-an-image-from-a-containerfile)
1. [Edit source code and mount in a pod](#edit-source-code-and-mount-in-a-pod)

## Root access inside containers

> THIS STEP SHOULD BE ONLY USED FOR DEVELOPING PURPOSES!!!

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

## Edit source code and mount in a pod

Follow these steps on your MicroShift instance

* Clone required project

NOTE: Do not create directory in HOME dir or other location, where
SELinux label might be not fine for containers.
NOTE: That step needs to be done on the Microshift host.

```sh
sudo mkdir -p /mnt/serviceDev ; sudo chmod 0777 /mnt/serviceDev
git clone https://opendev.org/zuul/nodepool /mnt/serviceDev/nodepool && cd /mnt/serviceDev/nodepool
git checkout 8.2.0
```

* Create local storageclass with name `manual`

```sh
kubectl apply -f - << EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: manual
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
EOF
```

* Create PV:

```sh
kubectl apply -f - << EOF
apiVersion: v1
kind: PersistentVolume
metadata:
  name: my-pv-volume
  labels:
    type: local
spec:
  storageClassName: manual
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  hostPath:
    path: "/mnt/serviceDev/nodepool/nodepool"
EOF
```

* Create PVC:

```sh
kubectl apply -f - << EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc-volume
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: manual
EOF
```

* And add the volume into the app:

```sh
kubectl edit deployment.apps/nodepool-launcher
```

End edit configuration to follow:

```yaml
(...)
spec:
  containers:
    ...
    securityContext:
      privileged: true
    volumeMounts:
    - name: host-mount
      mountPath: /usr/local/lib/python3.11/site-packages/nodepool
  ...
  securityContext: {}
  volumes:
    - name: host-mount
      persistentVolumeClaim:
        claimName: my-pvc-volume
```

For example output:

```yaml
    spec:
      ...
      containers:
      - image: quay.io/software-factory/nodepool-launcher:8.2.0-2 # E: wrong indentation: expected 8 but found 6
        name: launcher
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /usr/local/lib/python3.11/site-packages/nodepool
          name: host-mount
        # container is using root, so $HOME is /root, where .kube/config does not exists.
        - mountPath: /root/.kube/config
          name: nodepool-kubeconfig
          subPath: config
      ...
      securityContext: {}
      volumes:
      - name: host-mount
        persistentVolumeClaim:
          claimName: my-pvc-volume
```

Make sure, that `securityContext` are set as in the example!

Helpful [lecture](https://docs.openshift.com/container-platform/4.13/storage/persistent_storage/persistent_storage_local/persistent-storage-hostpath.html)
Also helpful would be change scc to anyuid with [example](https://examples.openshift.pub/deploy/scc-anyuid/)
