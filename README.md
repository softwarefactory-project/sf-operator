# sf-operator

**sf-operator** is the next version of [Software Factory](https://www.softwarefactory-project.io) distributed as an **OpenShift Operator**.

> Software Factory used to be a full Software Development Forge including services such as Gerrit or Etherpad, however the scope of this
new version is focused on the Continuous Integration components.

The Operator mainly manages a **SoftwareFactory** Custom Resource. This resource handles operations on the folowing services:

- [Zuul](https://zuul-ci.org/docs/zuul/latest)
- [Nodepool](https://zuul-ci.org/docs/nodepool/latest)
- Logserver (Zuul jobs artifacts storage)

The resource also deploys and maintains additional required services for Zuul and Nodepool such as:

- Zookeeper
- MariaDB

The **SoftwareFactory** CR provides a **Configuration as Code** workflow for Zuul and Nodepool. Indeed the instance must be
configured to rely on a Git repository hosted on a **Code Review System** to host non sensitive settings, such as the
[Zuul tenant configuration](https://zuul-ci.org/docs/zuul/latest/tenants.html#tenant) and the
[Nodepool configuration](https://zuul-ci.org/docs/nodepool/latest/configuration.html#configuration).

## Contacts

You can reach us on the [Software Factory Matrix channel](https://app.element.io/#/room/#softwarefactory-project:matrix.org).

## Status

The current project status is: **Alpha - DO NOT USE IN PRODUCTION**

See:

- the [CONTRIBUTING documentation](CONTRIBUTING.md) to discover how to hack on the project.
- the [CHANGELOG](CHANGELOG.md) to read about project development progress.
- the [ADRs](doc/adr/) to read about the Architecture Decision Records.

## Installation

### Prerequisites

The **sf-operator** is designed to run on Openshift and Openshift variants, thus you need an Openshift cluster to install it and make use of it.

Furthermore we package the operator via OLM so your cluster must have the [OLM system](https://olm.operatorframework.io/) running.

We currently validate the **sf-operator** on an installation of Microshift deployed via the [microshift-ansible-role](https://github.com/openstack-k8s-operators/ansible-microshift-role).

> As we are in early development phases it is recommended to use Microshift as your Openshift environment if you want to test the operator.

For more information about Microshift installation please refer to the [microshift/README.md](tools/microshift/README.md).

### Installing the CatalogSource

The **sf-operator** is packaged for OLM and is distributed by a **Catalog** we maintain. Thus to deploy the Operator you need to add
a new **CatalogSource** resource into the `olm` namespace.

Create the following CatalogSource:

```sh
cat << EOF | kubectl create -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: sf-operator-catalog
  namespace: olm
spec:
  sourceType: grpc
  image: quay.io/software-factory/sf-operator-catalog:latest
  displayName: sf-operator
  publisher: softwarefactory-project.io
  updateStrategy:
    registryPoll:
      interval: 60m
EOF
```

Once the Catalog is defined, we can tell OLM to install the **sf-operator** by applying a **Subscription** resource:

```sh
cat << EOF | kubectl create -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: sf-operator-sub
  namespace: operators
spec:
  channel: preview
  name: sf-operator
  source: sf-operator-catalog
  sourceNamespace: olm
EOF
```

After few seconds you should see that the operator is running:

```sh
kubectl -n operators get deployment.apps/sf-operator-controller-manager
NAME                                                READY   UP-TO-DATE   AVAILABLE   AGE
pod/sf-operator-controller-manager-65cc95995c-bgt25   2/2     Running   0          3m49s
```

Custom Resources supported by the sf-operator could be listed with the following command:

```sh
kubectl get crd -o custom-columns=NAME:.metadata.name | grep softwarefactory-project.io
logservers.sf.softwarefactory-project.io
softwarefactories.sf.softwarefactory-project.io
```

### Start a SoftwareFactory instance

Let's create a namespace where we'll reclaim a **SoftwareFactory** resource.

> Currently, the namespace must allow `privileged` containers to run. Indeed the `zuul-executor` container requires
extra priviledges due to the use of [bubblewrap](https://github.com/containers/bubblewrap).

For this example we will create **sf** namespace.

```sh
kubectl create namespace sf
kubectl label --overwrite ns sf pod-security.kubernetes.io/enforce=privileged
kubectl label --overwrite ns sf pod-security.kubernetes.io/enforce-version=v1.24
oc adm policy add-scc-to-user privileged system:serviceaccount:sf:default
```

Now we are ready to create our **SoftwareFactory** instance via OLM:

> Only one instance of a **SoftwareFactory** resource by namespace is recommanded in order to avoid resources naming collapse.

```sh
cat << EOF | kubectl -n sf create -f -
apiVersion: sf.softwarefactory-project.io/v1
kind: SoftwareFactory
metadata:
  name: my-sf
  namespace: sf
spec:
  fqdn: "sftests.com"
EOF
```

After few minutes, you should see a `READY` resource called `my-sf`:
```sh
kubectl -n sf get sf
NAME    READY
my-sf   true
```


The following `Routes` (or `Ingress`) are created:

```
kubectl -n sf get routes -o custom-columns=HOST:.spec.host

HOST
zuul.sftests.com
logserver.sftests.com
nodepool.sftests.com
```

At that point you have successfully deployed a **SoftwareFactory** instance. The Zuul web UI is accessible using the following URL: https://zuul.sftests.com

To finalize the setup you'll need to setup a **config** repository for enabling the **config as code** workflow. Please read the following guides.

## How to set the config repository

To setup the **Configuration As Code** workflow, a dedicated Git repository and a *bot* account must exist on **Gerrit**. See the "Requirements for the config repository on Gerrit" section below.

> Currently, the **sf-operator** only supports a **config** respository hosted on Gerrit code review system.

First, a Zuul connection to the Gerrit code review system must be defined via the **SoftwareFactory**'s Spec:

```sh
kubectl edit sf my-sf
...
spec:
  zuul:
    gerritconns:
      - name: gerrit
        username: zuul
        hostname: review.rdoproject.org
        puburl: "https://review.rdoproject.org/r"
...
```

Checkout the [CRD's OpenAPI schema](config/crd/bases/sf.softwarefactory-project.io_softwarefactories.yaml) for specification details.

Then, specify the **config** repository location:

```sh
kubectl edit sf my-sf
...
spec:
  config-location:
    base-url: "https://review.rdoproject.org/r"
    name: config
    zuul-connection-name: gerrit
...
```

Wait for the 'my-sf' resource to become *READY*:

```sh
kubectl get sf my-sf -o jsonpath='{.status}'

{"observedGeneration":1,"ready":true}
```

Once ready, the **config** repository must appear in the [Zuul's internal tenant projects list](https://zuul.sftests.com/t/internal/projects).
Furthermore, the **config-check** and **config-update** are set automatically to run the internal tenant's **check**, **gate** and **post** pipelines.

Any changes opened on the **config** repository will trigger the **config-check** job. The job validates the proposed configuration and applies them to Zuul and Nodepool.

### Requirements for the config repository on Gerrit

Zuul needs to authenticate on Gerrit, then a bot account must be set and zuul must be able to connect as SSH to Gerrit.

The Zuul SSH public key can be fetch from the **zuul-ssh-key** secret:

```sh
kubectl get secret zuul-ssh-key -o jsonpath={.data.pub} | base64 -d
```

The **config** repository must be set with specific ACLs and Labels. Zuul expects to report, trigger and merge based on specific labels values.

Here are the required labels to define in the **config** repository's *Access* settings (*meta/config*) on Gerrit:

```INI
[label "Code-Review"]
	function = MaxWithBlock
	defaultValue = 0
	copyMinScore = true
	copyAllScoresOnTrivialRebase = true
	value = -2 Do not submit
	value = "-1 I would prefer that you didn't submit this"
	value = 0 No score
	value = +1 Looks good to me, but someone else must approve
	value = +2 Looks good to me (core reviewer)
	copyAllScoresIfNoCodeChange = true
[label "Verified"]
	value = -2 Fails
	value = "-1 Doesn't seem to work"
	value = 0 No score
	value = +1 Works for me
	value = +2 Verified
[label "Workflow"]
	value = -1 Work in progress
	value = 0 Ready for reviews
	value = +1 Approved
```

Here are the required ACLs (Assuming the zuul bot user is part of the 'Service Users' group):

```INI
[access "refs/heads/*"]
label-Verified = -2..+2 group Service Users
submit = group Service Users
```

For further information check the Zuul documentation's [Gerrit section](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#gerrit).

## How to configure Zuul via the config repo
## How to configure Nodepool via the config repo

### Using cloud image from openstack cloud

* on the config repo, edit nodepool/nodepool.yaml to add labels and providers

```yaml
labels:
-   name: cloud-centos-9-stream
    # min-ready: 1
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

* propose a review and merge the change

* if min-ready is set to 1, you can validate an instance is available with:

```sh
$ kubectl exec -ti nodepool-launcher-$uuid -c launcher -- nodepool list
+------------+----------+-----------------------+--------------------------------------+--------------+------+-------+-------------+----------+
| ID         | Provider | Label                 | Server ID                            | Public IPv4  | IPv6 | State | Age         | Locked   |
+------------+----------+-----------------------+--------------------------------------+--------------+------+-------+-------------+----------+
| 0000000001 | default  | cloud-centos-9-stream | 6b9c0efb-493d-442b-bb2f-550ffcdb3fb3 | $public_ip   |      | ready | 00:00:01:27 | unlocked |
+------------+----------+-----------------------+--------------------------------------+--------------+------+-------+-------------+----------+
```

* you can validate zuul-executor can connect to the instance with:

```sh
$kubectl exec -ti zuul-executor-0 -- ssh -o "StrictHostKeyChecking no" -i /var/lib/zuul-ssh/..data/priv cloud-user@$public_ip hostname
Warning: Permanently added '$public_ip' (ED25519) to the list of known hosts.
np0000000001
```
## How to set the Nodepool kubeconfig or clouds.yaml

Providers secrets configuration such as `kubeconfig` or `clouds.yaml` needed by `Nodepool` services
must be set into the `nodepool-providers-secrets`'s `Secret` resource.

The `sfconfig` CLI provides the `nodepool-providers-secrets` command to `update` and `dump` the
`Secret` content.

The command reads or writes files content defined into `nodepool.clouds_file` and `nodepool.kube_file`
section of `sfconfig.yaml`

To add or modify the providers secrets, first, you have to set the files contents, then run:

```sh
tools/sfconfig nodepool-providers-secrets --upload
```

The `SoftwareFactory` CR will pass into a "non Ready" state until all Nodepool services are
restarted.

The `dump` command, fetches the current content of the secret and updates the local files.

## How to setup TLS for exposed Services route

### How to use pre-existing X509 certificates

The operator watches specifics `Secret` in the `SoftwareFactory` Custom Resources namespace.
When those secrets' data hold a Certificate, Key and CA Certificate (following a specific scheme) then
the sf-operator is able to reconfigure the corresponding service `Route`'s TLS to use the TLS material
contained into the secret.

The `sfconfig` command can be used to setup secrets.

> The `create-service-ssl-secret` is verifying the SSL certificate/key before updating the `Secret`.

The example below updates the `Secret` for the `logserver` service. The `SoftwareFactory` CR will
pass into a "non Ready" state until the `Route` is reconfigured. Once `Ready`, the `Route` will
serve the new Certificate.

```sh
./tools/sfconfig create-service-ssl-secret \
    --sf-service-ca /tmp/ssl/localCA.pem \
    --sf-service-key /tmp/ssl/ssl.key \
    --sf-service-cert /tmp/ssl/ssl.crt \
    --sf-service-name logserver
```

Expected `sf-service-name` value are:

  - logserver
  - zuul
  - nodepool

### How to use Let's Encrypt Certificates

The operator offers an option to request Certificates from `Let's Encrypt` using the `ACME http01`
challenge. All DNS names exposed by the `Routes` must be publicly resolvable.

`sf-operator` relies on the `cert-manager` operator to handle this setup.

> When enabled, TLS material provided via `Secret`, are not used anymore.

```sh
kubectl edit sf my-sf
...
spec:
  letsEncrypt:
    server: "staging"
...
```

The `SoftwareFactory` CR will pass into a "non Ready" state until all `Challenge` are resolved
and all the `Route` are reconfigured.

> Set the server to `prod` to use the Let's Encrypt production server.

> Routes will be re-configured when Certificates are renewed.

## Using zuul-client

### As the deployment owner

The `sfconfig` tool eases the use of `zuul-client` by directly calling the zuul-client program from a running Zuul web pod.

To get access to zuul-client tool run, from the root of the project:

```bash
./tools/sfconfig zuul-client -h
```