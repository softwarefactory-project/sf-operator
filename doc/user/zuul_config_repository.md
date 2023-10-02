# Zuul configuration

## Table of Contents

1. [File structure](#file-structure)
1. [Defining a Zuul tenant](#defining-a-zuul-tenant)
1. [Integrating a repository with a Zuul tenant](#integrating-a-repository-with-a-zuul-tenant)

## File structure

When the `config-check` and `config-update` jobs are run on git events occurring on the config repository, the following file structure is expected:

```
|_ zuul/
       |_ main.yaml
```

> if the file structure is missing or partial the jobs will skip the related configuration check and update.

The file `zuul/main.yaml` holds the [tenants configuration](https://zuul-ci.org/docs/zuul/latest/tenants.html) that will be applied the deployment.

> A git repository is called a `project` within the Zuul terminology.

## Defining a Zuul tenant

A Zuul tenant is defined through the `zuul/main.yaml` file. A Zuul tenant holds an isolated Zuul configuration from others Zuul tenants.

The configuration provided into `zuul/main.yaml` will be appended to the [base configuration](../../controllers/static/zuul/generate-tenant-config.sh).

> Please take care not to override the `internal` tenant definition.

Here is an example of a minimal tenant defintion:

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

A minimal Zuul tenant should be populated with at least one [config/trusted](https://zuul-ci.org/docs/zuul/latest/tenants.html#attr-tenant.config-projects) project.
This tenant's config project (`my-tenant-config-repo` in the example above) defines Zuul configuration such as the `pipelines`, the `base job` and related base Ansible `playbooks`.

While the tenant's config project could be setup manually, we also provide a `cli` command to scaffold its content.

> Note that [zuul/zuul-jobs](https://zuul-ci.org/docs/zuul-jobs/latest/) should be part of a new tenant. The `bootstrap-tenant-config-repo` command expects that
  this repository is part of the tenant.

> The `opendev.org` connection is available by default on any Software-Factory deployment.

### Bootstrap a config-project

sfconfig allows you to create a scaffolding for a new tenant's config repository. It defines:

* the `check`, `gate` and `post` pipelines
* the `base job` and `playbooks`

> The tool only supports the definition of `pipelines` compatible with a `Gerrit` connection.

Get a local checkout of the tenant's config project/repository then run:

```sh
./tools/sfconfig bootstrap-tenant-config-repo --connection [connection] --outpath [/path/to/repository]
```

### Modify and merge

The scaffolding is yours to modify to suit your needs. Once you are happy with your changes, commit them and push them to your code review system.

If all went well you should see the `pipelines` appear in the `Zuul status page` for the related `tenant`. Also check for any tenant configuration problems by clicking on the `blue bell` on the Zuul web UI. Fix problems by pushing new commits into the repository until the tenant configuration errors page is clear.

### Next steps ?

The Zuul tenant is now ready to be used. Other repositories can be added to the tenant definition (see the [tenants configuration](https://zuul-ci.org/docs/zuul/latest/tenants.html) documentation).

## Integrating a repository with a Zuul tenant

To integrate a repository inside a Zuul tenant, first the [tenant must have been created](#defining-a-zuul-tenant) then the repository must be added into the list of `config/trusted` or `untrusted` repositories for a given `zuul connection`.

Zuul might be configured to run jobs on this new repository, then make sure that the
Zuul bot account for the related connection is authorized to set approvals, reports comments, (optionaly merge) changes (Github PRs, Gitlab MRs, Gerrit Reviews).

See the related section by connection type:

- Gerrit: [Set the Gerrit ACLs for repository](../deployment/config_repository#repository-acls-and-labels)