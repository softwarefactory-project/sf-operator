# Zuul configuration


1. [File structure](#file-structure)
1. [Defining a Zuul tenant](#defining-a-zuul-tenant)
1. [Integrating a repository with a Zuul tenant](#integrating-a-repository-with-a-zuul-tenant)

## File structure

When the `config-check` and `config-update` jobs are run on git events occurring on the config repository, the following file structure is expected:

```
|_ zuul/
       |_ main.yaml
```

!!! note
    if the file structure is missing or partial, the jobs will skip the related configuration check and update.

The file `zuul/main.yaml` holds the [tenants configuration](https://zuul-ci.org/docs/zuul/latest/tenants.html) that will be applied to the deployment.

!!! info "Definition"
    A Git repository is called a `project` within the Zuul terminology.

## Defining a Zuul tenant

A Zuul tenant is defined through the `zuul/main.yaml` file. A Zuul tenant holds a Zuul configuration that is isolated from other Zuul tenants.

The configuration provided in `zuul/main.yaml` will be appended to the base configuration.

??? question "What happens during a `config-update` job?"

    When a change to `zuul/main.yaml` is merged, the following script is run to update Zuul's tenant configuration on the scheduler:

    ``` title="controllers/static/zuul/generate-tenant-config.sh"
    --8<-- "controllers/static/zuul/generate-tenant-config.sh"
    ```

Here is an example of a minimal tenant definition:

```yaml
- tenant:
    name: my-new-tenant
    source:
      my-gerrit-connection:
        config-projects:
          - my-tenant-config-repo
        untrusted-projects: []
      opendev.org:
        untrusted-projects:
          - zuul/zuul-jobs
```

!!! danger
    Please take care not to override the `internal` tenant definition.

A minimal Zuul tenant should be populated with at least one [config/trusted](https://zuul-ci.org/docs/zuul/latest/tenants.html#attr-tenant.config-projects) project.
This tenant's config project (`my-tenant-config-repo` in the example above) defines the Zuul configuration, such as the `pipelines`, the `base job`, and related base Ansible `playbooks`.

While the tenant's config project could be set up manually, we also provide a `cli` command to scaffold its content.

!!! note
    The [zuul/zuul-jobs](https://zuul-ci.org/docs/zuul-jobs/latest/) project should always be part of a new tenant. The `SF bootstrap-tenant` command expects that
    this repository is part of the tenant.

!!! info
    The `opendev.org` connection is preconfigured on any Software Factory deployment if it is not user-defined in
    the SoftwareFactory Spec Zuul connections.

### Bootstrap a config-project

[`sf-operator`](./../reference/cli/index.md#bootstrap-tenant) allows you to create a scaffolding for a new tenant's config repository. It defines:

* the `check`, `gate` and `post` pipelines
* the `base job` and `playbooks`

!!! warning
    The tool only supports the definition of `pipelines` that are compatible with `Gerrit` and `GitLab` connections.

Get a local checkout of the tenant's config project/repository, and then run:

```sh
sf-operator SF bootstrap-tenant </path/to/repository> --connection [connection]
```

### Modify and merge

The scaffolding is yours to modify to suit your needs. Once you are happy with your changes, commit them and push them to your code review system.

If all went well, you should see the `pipelines` appear in the `Zuul status page` for the related `tenant`. Also, check for any tenant configuration problems by clicking on the `blue bell` on the Zuul web UI. Fix problems by pushing new commits into the repository until the tenant configuration errors page is clear.

### Next steps?

The Zuul tenant is now ready to be used. Other repositories can be added to the tenant definition (see the [tenants configuration](https://zuul-ci.org/docs/zuul/latest/tenants.html) documentation).

## Integrating a repository with a Zuul tenant

To integrate a repository inside a Zuul tenant, first, the [tenant must have been created](#defining-a-zuul-tenant), and then the repository must be added to the list of `config/trusted` or `untrusted` repositories for a given `zuul connection`.

Zuul might be configured to run jobs on this new repository; then, make sure that the
Zuul bot account for the related connection is authorized to set approvals, report comments, and (optionally) merge changes (GitHub PRs, GitLab MRs, Gerrit Reviews).

See the related section by connection type:

- Gerrit: [Set the Gerrit ACLs for repository](../deployment/config_repository.md#repository-acls-and-labels)
