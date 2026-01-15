# Nodepool configuration


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

!!! note
    If the file structure is missing or partial, the jobs will skip the related configuration check and update.

The file `nodepool.yaml` holds [the labels and node providers configuration](https://zuul-ci.org/docs/nodepool/latest/configuration.html). This configuration is used by the `nodepool-launcher` process.

The file `nodepool-builder.yaml` holds [the diskimages and provider's image configuration](https://zuul-ci.org/docs/nodepool/latest/configuration.html). This configuration is used by the `nodepool-builder` process.

The `dib-ansible` directory is used by `nodepool-builder` as the image build definition directory.

## Configuring Nodepool launcher

!!! danger
    Please take care not to override any of the base settings!

The configuration provided in `nodepool/nodepool.yaml` will be appended to the base configuration.

??? question "What happens during a `config-update` job?"

    When a change to nodepool's configuration is merged, the following script is run to update the pods running nodepool:

    ```bash title="controllers/static/nodepool/generate-config.sh"
    --8<-- "controllers/static/nodepool/generate-config.sh"
    ```

For each provider used in the Nodepool launcher configuration, Nodepool must be able to find the required connection credentials. Please refer to the deployment documentation about [setting up provider secrets](../deployment/nodepool.md#setting-up-providers-secrets).

### Use an official cloud image within an OpenStack cloud

There is a simple way to configure Nodepool to use a cloud image with Zuul's SSH key so that Zuul can run jobs on images in an OpenStack cloud.

1. Edit `nodepool/nodepool.yaml` to add labels and providers:

```yaml
labels:
-   name: cloud-c9s
    min-ready: 1
providers:
- name: default
  cloud: default
  clean-floating-ips: true
  image-name-format: '{image_name}-{timestamp}'
  boot-timeout: 120 # default 60
  cloud-images:
    - name: cloud-centos-9-stream
      username: cloud-user
  pools:
    - name: main
      max-servers: 10
      networks:
        - $public_network_name
      labels:
        - cloud-image: cloud-centos-9-stream
          name: cloud-cs9
          flavor-name: $flavor
          userdata: |
            #cloud-config
            package_update: true
            users:
              - name: cloud-user
                ssh_authorized_keys:
                  - $zuul-ssh-key
                  - $zuul-spare-ssh-key
```

2. Save, commit, propose a review and merge the change.
3. Wait for the `config-update` job to complete.
4. If the `min-ready` property is over 0, you should see the new label and
   a ready node under the `labels` and `nodes` pages in the Zuul web UI.

!!! tip
    If you encounter issues, please refer to the [troubleshooting guide](../deployment/nodepool.md#troubleshooting).

## Configuring Nodepool builder

!!! danger
    Please take care not to override any of the base settings!

The configuration provided in `nodepool/nodepool-builder.yaml` will be appended to the base configuration. (1)
{ .annotate }

1. See ["What happens during a `config-update` job?"](#configuring-nodepool-launcher) for implementation details.

For each provider used in the Nodepool builder configuration, Nodepool must be able to find the required connection credentials. Please refer to the deployment documentation about [setting up provider secrets](../deployment/nodepool.md#setting-up-providers-secrets).

### disk-image-builder

Due to the security restrictions related to the OpenShift platform, the use of [disk-image-builder](https://docs.openstack.org/diskimage-builder/) is not possible. Thus, we do not recommend its usage in the context of the `sf-operator`.

### dib-ansible

`dib-ansible` (1) is an alternative [dib-cmd](https://zuul-ci.org/docs/nodepool/latest/configuration.html#attr-diskimages.dib-cmd) wrapper that we provide within the `sf-operator` project. It is a `dib-cmd` wrapper for the `ansible-playbook` command.
{ .annotate }

1. For implementation details, the wrapper can be found at [controllers/static/nodepool/dib-ansible.py](https://raw.githubusercontent.com/softwarefactory-project/sf-operator/master/controllers/static/nodepool/dib-ansible.py)

We recommend using `dib-ansible` to externalize the image build process on at least one image builder machine.

To define a `diskimage` using `dib-ansible`, use the following in `nodepool/nodepool-builder.yaml`:

```yaml
diskimages:
  - dib-cmd: /usr/local/bin/dib-ansible my-cloud-image.yaml
    formats:
      - raw
    name: my-cloud-image
    username: zuul-worker
```

The image build playbook file `my-cloud-image.yaml` must be defined in the `nodepool/dib-ansible/` directory.

Here is an example of an image build playbook:

```yaml
- name: My cloud image build playbook
  hosts: image-builder
  vars:
    built_image_path: /var/lib/builder/cache/my-cloud-image
  tasks:
    - debug:
        msg: "Building {{ image_output }}"
    - name: Copy the Zuul public key on the image-builder to integrate it on the built cloud image
      copy:
        src: /var/lib/zuul-ssh-key/pub
        dest: /tmp/zuul-ssh-key.pub
    - name: Copy the Zuul spare public key on the image-builder to integrate it on the built cloud image
      copy:
        src: /var/lib/zuul-spare-ssh-key/pub
        dest: /tmp/zuul-spare-ssh-key.pub
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
    # Synchronize the image back from the image-builder to the nodepool-builder
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

!!! note
    Zuul needs to authenticate via SSH on Virtual Machines spawned from built cloud images. Thus, the Zuul SSH public key should be added as
    an authorized key for the user to which Zuul will connect. The Zuul SSH public key is available on the `nodepool-builder` in the file
    `/var/lib/zuul-ssh-key/pub`. A cloud image build playbook can read that file to prepare a cloud image.

Finally, we need an `inventory.yaml` file. It must be defined in `nodepool/dib-ansible/inventory.yaml`:

```yaml
ungrouped:
  hosts:
    image-builder:
      ansible_host: <my-builder-ip-or-hostname>/
      ansible_user: nodepool
```

!!! note
    Nodepool builder must be able to connect via SSH to your image-builder machine. Thus, please refer to the section [Get the Nodepool builder SSH public key](../deployment/nodepool.md#get-the-builders-ssh-public-key).

Once these three files, `nodepool/dib-ansible/inventory.yaml`, `nodepool/dib-ansible/my-cloud-image.yaml`, and `nodepool/nodepool-builder.yaml`, are merged into the Software Factory `config` repository and the `config-update` has succeeded, then Nodepool will run the build process.

??? tip "SSH connection issues with an image-builder host?"
    At the first connection attempt of the `nodepool-builder` to an `image-builder` host, Ansible will refuse to connect because the SSH host key is not known. Please refer to the section [Accept an image-builder's SSH Host key](../deployment/nodepool.md#accept-an-image-builders-ssh-host-key).

The image build status can be consulted by accessing this endpoint: `https://<fqdn>/nodepool/api/dib-image-list`.

The image build logs can be consulted by accessing this endpoint: `https://<fqdn>/nodepool/builds/`.