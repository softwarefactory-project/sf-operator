# Nodepool configuration

## Table of Contents

1. [File structure](#file-structure)
1. [Configuring Nodepool launcher](#configuring-nodepool-launcher)
1. [Configuring Nodepool builder](#configuring-nodepool-builder)

## File structure

When the `config-check` and `config-update` jobs are run on git events occurring on the config repository, the following file structure is expected:

```
/
|_ nodepool/
           |_ nodepool.yaml
	       |_ nodepool-builder.yaml
           |_ dib-ansible/
                         |_ inventory.yaml
                         |_ my-cloud-image-1.yaml
```

> if the file structure is missing or partial the jobs will skip the related configuration check and update.

The file `nodepool.yaml` holds [the labels and node providers configuration](https://zuul-ci.org/docs/nodepool/latest/configuration.html). This configuration is used by the `nodepool-launcher` process.

The file `nodepool-builder.yaml` holds [the diskimages and provider's image configuration](https://zuul-ci.org/docs/nodepool/latest/configuration.html). This configuration is used by the `nodepool-builder` process.

The `dib-ansible` directory is used by `nodepool-builder` as the images build definition directory.

## Configuring Nodepool launcher

The configuration provided in `nodepool/nodepool.yaml` will be appended to the [base configuration](../../controllers/static/nodepool/generate-config.sh).

> Please take care not to override any of the base settings

For each provider used in the Nodepool launcher configuration, nodepool must be able to find the required connection credentials. Please refer the deployment documentation about [setting up provider secrets](../deployment/nodepool#setting-up-provider-secrets).

### Use an official cloud image within an OpenStack cloud

There is a simple way to configure Nodepool to use a cloud image with Zuul's SSH key, so that Zuul can run jobs on images in an OpenStack cloud.

1. Edit `nodepool/nodepool.yaml` to add labels and providers:

```yaml
labels:
-   name: my-cloud-image-label
    min-ready: 1
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

2. Save, commit, propose a review and merge the change.
3. Wait for the `config-update` job to complete.
4. If the `min-ready` property is over 0, you should see in the Zuul web UI, the new label and
   a ready node under the `labels` and `nodes` pages.

> Please refer to the [troubleshooting guide](../deployment/nodepool#troubleshooting) if needed.

## Configuring Nodepool builder

The configuration provided in `nodepool/nodepool-builder.yaml` will be appended to the [base configuration](../../controllers/static/nodepool/generate-config.sh).

> Please take care not to override any of the base settings

For each provider used in the Nodepool builder configuration, nodepool must be able to find the required connection credentials. Please refer the deployment documentation about [setting up provider secrets](../deployment/nodepool#setting-up-provider-secrets).

### disk-image-builder

Due to the security restrictions related to the OpenShift platform, the use of [disk-image-builder](https://docs.openstack.org/diskimage-builder/) is not possible. Thus we do not recommand its usage in the context of the `sf-operator`.

### dib-ansible

[dib-ansible](../../controllers/static/nodepool/dib-ansible.py) is an alternative [dib-cmd](https://zuul-ci.org/docs/nodepool/latest/configuration.html#attr-diskimages.dib-cmd) wrapper that we provide within the `sf-operator` project. It is a `dib-cmd` wrapper to the `ansible-playbook` command.

We recommend using `dib-ansible` to externalize the image build process on at least one image builder machine.

To define a `diskimage` using `dib-ansible` use the following in `nodepool/nodepool-builder.yaml`:

```yaml
diskimages:
  - dib-cmd: /usr/local/bin/dib-ansible my-cloud-image.yaml
    formats:
      - raw
    name: my-cloud-image
    username: zuul-worker
```

The image build playbook file `my-cloud-image.yaml` must be defined into the `nodepool/dib-ansible/` directory.

Here is an example of an image build playbook:

```yaml
- name: My cloud image build playbook
  hosts: image-builder
  vars:
    built_image_path: /var/lib/builder/cache/my-cloud-image
  tasks:
    - debug:
        msg: "Building {{ image_output }}"
    # Build steps begin from here
    # - name: Build task 1
    #   shell: true
    # - name: Build task 2
    #   shell: true
    # Build steps end here
    # Set final image path based on the expected image type
    - set_fact:
        final_image_path: "{{ image_output }}.raw"
      when: raw_type | default(false)
    - set_fact:
        final_image_path: "{{ image_output }}.qcow2"
      when: qcow2_type | default(false)
    # Synchronize back the image from the image-builder to the nodepool-builder
    - ansible.posix.synchronize:
        mode: pull
        # src: is on the image-builder
        src: "{{ built_image_path }}"
        # dest: is on the nodepool-builder pod
        dest: "{{ final_image_path }}"
```

Here are the available variables and their meaning:

- image_output: contains the path of the image the builder expects to find under its build directory. The file suffix is not part of the provided path.
- qcow2_type: is a boolean specifying if the built image format is `qcow2`.
- raw_type: is a boolean specifying if the built image format is `raw`.


Finally we need an `inventory.yaml` file. It must be defined into `nodepool/dib-ansible/inventory.yaml`:

```yaml
ungrouped:
  hosts:
    image-builder:
      ansible_host: <my-builder-ip-or-hostname>
      ansible_user: nodepool
```

> Nodepool builder must be able to connect via SSH to your image-builder machine. Thus please refer to the section [Get the Nodepool builder SSH public key](../deployment/nodepool#get-the-builders-ssh-public-key).

Once these three files `nodepool/dib-ansible/inventory.yaml`, `nodepool/dib-ansible/my-cloud-image.yaml` and `nodepool/nodepool-builder.yaml` are merged into the Software Factory `config` repository and the `config-update` has succeeded then Nodepool will run the build proces.

> At the first connection attempt of the `nodepool-builder` to an `image-builder` host, Ansible will refuse to connect because the SSH Host key is not known. Please refer to the section [Accept an image-builder's SSH Host key](../deployment/nodepool#accept-an-image-builders-ssh-host-key).

The image builds status can be consulted by accessing this endpoint: `https://nodepool.<fqdn>/dib-image-list`.

The image builds logs can be consulted by accessing this endpoint: `https://nodepool.<fqdn>/builds`.