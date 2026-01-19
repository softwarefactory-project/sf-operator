# Config Repository


1. [Concept](#concept)
1. [Setting up the repository](#setting-up-the-repository)
    1. [Gerrit](#hosting-on-gerrit)
        - [Prerequisites](#prerequisites-on-gerrit)
        - [Configuring Gerrit](#configuring-gerrit)
        - [Configuring the Zuul connection](#configuring-the-gerrit-zuul-connection)
    1. [GitLab](#hosting-on-gitlab)
        - [Configuring the Zuul connection](#configuring-the-gitlab-zuul-connection)
1. [Next Steps](#next-steps)

## Concept

!!! note
    If you are already familiar with the config repository in Software Factory, you can skip this section and go straight to [setting up the repository](#setting-up-the-repository).

In Software Factory, Zuul's and Nodepool's configurations are stored in a special git repository called the **config repository**. Zuul is also pre-configured to run the following CI pipelines and jobs against changes on this repository, in a reserved tenant (`internal`):

| Pipeline | When | Jobs  |
|-----------|-----|------|
| **check**   | A change is opened for review on the config repository | **config-check** |
| **gate**     | A change on the config repository is scheduled for merging, before merging | **config-check** |
| **post**     | After a change is merged | **config-update** |

The `internal` tenant configuration is stored in a hidden Git repository and managed solely by the SF-Operator.
Below you can find more details about the default jobs running against the config repository:

| Job name | Description |
|----------|--------------|
| **config-check** | runs `zuul-admin tenant-conf-check` and `nodepool config-validate` to check the service configurations for errors |
| **config-update** | applies the new configurations and reconfigures the services as needed |

This setup enables GitOps workflows on the configuration of your Zuul-based CI infrastructure.

The config repository is expected to follow a specific file structure for the automation to work properly. Please refer to the [user documentation](../user/index.md)
to learn more about the expected repository file structure and its usage.

Any other file or folder will be ignored.

## Setting up the repository

As of the current version of the SF-Operator, Gerrit and GitLab are the only supported hosting options for the config repository.

!!! note
    You can follow the [developer's documentation to deploy a test Gerrit instance](../developer/howtos/index.md#gerrit) if needed.

### Hosting on Gerrit

#### Prerequisites on Gerrit

1. Make sure that the deployment and the Gerrit host can communicate, especially via Gerrit's SSH port (usually TCP/29418).
2. Make sure that you can create accounts on the Gerrit host, or at least set their SSH public key.
3. Make sure that you can create a project on the Gerrit host, or at least modify its configuration.

#### Configuring Gerrit

##### Gerrit Bot account

Zuul needs a bot account on Gerrit with SSH access to be able to subscribe to events and merge changes to the config repository.

The SF-Operator automatically creates an SSH key pair that can be used by such bot accounts and stores it in a secret called `zuul-ssh-key`.

You can create the Zuul bot account on Gerrit with this convenient one-liner (assuming your deployment is installed in the `sf` namespace):

```sh
kubectl get secret zuul-ssh-key -n sf -o jsonpath={.data.pub} | base64 -d | ssh -p29418 <gerrit_host> gerrit create-account --ssh-key - --full-name Zuul --email zuul@example.com zuul
```

Then add the bot account to the `Service Users` group:

```sh
ssh -p29418 <gerrit_host> gerrit set-members --add zuul@example.com "Service Users"
```

##### Repository ACLs and Labels

!!! note
    Access controls and label management with Gerrit is out of the scope of this documentation. Please refer to
    Gerrit's documentation for further details, for example
    [here](https://gerrit-review.googlesource.com/Documentation/access-control.html) for ACLs
    or [here](https://gerrit-review.googlesource.com/Documentation/config-labels.html) for labels.

The repository must be set with specific ACLs to allow Zuul to interact with it:

```INI
[access "refs/heads/*"]
label-Verified = -2..+2 group Service Users
submit = group Service Users
```

Zuul triggers events based on specific labels, so these must be configured as well.

Here are the required labels to define in the repository's *Access* settings (*meta/config*) on Gerrit:

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

For further information check the [Gerrit section](https://zuul-ci.org/docs/zuul/latest/drivers/gerrit.html#gerrit) in Zuul's documentation.

#### Configuring the Gerrit Zuul connection

In order for Zuul to start listening to Gerrit events, add a `gerritconn` property in your deployed **SoftwareFactory**'s spec. Edit the spec with:

```sh
kubectl edit sf my-sf
```

```yaml
[...]
spec:
  zuul:
    gerritconns:
      - name: gerrit
        username: zuul # (1)
        hostname: <gerrit_ssh_hostname>
        puburl: "<gerrit_url>"
[...]
```

1. The `username` is the name of the [bot account that was set up in the previous section](#gerrit-bot-account).

You can check the [CRD's OpenAPI schema](./crds.md) for specification details.

At that step, you can continue the setting by [configuring the location of the config repository](#set-the-config-repository-location).

### Hosting on GitLab

Zuul needs:

* an API token to communicate with the GitLab API
* a webhook token in order to authenticate the webhook payloads sent by the GitLab instance.

It is advised to request a bot account from your GitLab admin, especially if Zuul must act on repositories spread across multiple
Gitlab project groups. However if all repositories are located inside the same project's group, then a simple group's token
is sufficient.

Please refer to the [upstream Zuul documentation for more information about the GitLab driver and how to define
an API token and webhook token](https://zuul-ci.org/docs/zuul/latest/drivers/gitlab.html#gitlab).

!!! note
    The webhook URL to configure on the project or on the project's group setting must
    be `https://<fqdn>/zuul/api/connection/<zuul-connection-name>/payload`

The `gate` pipeline defined for the `config` repository workflow relies on the `workflow` label for the pipeline
trigger rule. Thus, a GitLab label named `workflow` must be defined in the `Settings` of the `config` repository.

#### Configuring the Gitlab Zuul connection

To set up the Zuul connection to the GitLab instance, you first need a `Secret` resource (eg. named `gitlab-com-secret`)
to store the API and webhook tokens. The `Secret`'s scheme is as follows:

```yaml
kind: Secret
apiVersion: v1
metadata:
  name: gitlab-conn-secret
  namespace: sf
type: Opaque
data:
  api_token: <api-token-in-base64>
  webhook_token: <web-hook-token-in-base64>
```

This `Secret` resource must be applied to the same `Namespace` as the `SoftwareFactory` resource.

Then, edit the SoftwareFactory's CR:

```sh
kubectl edit sf my-sf
[...]
spec:
    zuul:
      gitlabconns:
        - name: <zuul-connection-name>
          server: gitlab.com
          baseurl: https://gitlab.com
          secrets: gitlab-conn-secret
[...]
```

At that step, you can continue the setting by [configuring the location of the config repository](#set-the-config-repository-location).

### Set the config repository location

Specify the config repository location (adapt according to your connection/repository name and location):

```sh
kubectl edit sf my-sf
[...]
spec:
  config-location:
    name: <config repository name>
    zuul-connection-name: gerrit
[...]
```

Wait for the 'my-sf' resource to become *READY*:

```sh
kubectl get sf my-sf -o jsonpath='{.status}'

{"observedGeneration":1,"ready":true}
```

Once the resource is ready, the config repository will appear listed on the internal tenant's projects page at `https://<FQDN>/zuul/t/internal/projects` .

## Next Steps

You may now want to configure [connection secrets for nodepool providers](./nodepool.md) (kubeconfig, clouds.yaml).

## How to be an administrator in Internal Tenant

As stated in the [Concept](#concept) section, the SF-Operator manages a hidden Git repository that defines and sets the
`internal` tenant.

This tenant has an `admin-rules` definition, setting the `admin-internal` group as the tenant administrator.

To be an `internal` administrator, just set an [`authorization-rule`](https://zuul-ci.org/docs/zuul/latest/tenants.html#authorization-rule) named `admin-internal` in the config project defined in `Set the config repository location`
