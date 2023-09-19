# Getting started

This document presents a way to bootstrap actions on Zuul for your repository.

## Table of Contents

1. [Integrating a repository with Zuul](#integrating-a-repository-with-zuul)

## Integrating a repository with Zuul

### Prerequisites

* The repository must have been added as a project in a Zuul tenant. Zuul tenants are managed in the [config repository](./../deployment/config_repository.md). If needed, clone the repository, submit a patch adding the project, and get the patch merged.
* You must know the name of the connection used by Zuul to listen to git events on your repository. This can be found in the [config repository](./../deployment/config_repository.md) as well, or in the `projects` page on the Zuul Web GUI.

### create Zuul scaffolding

sfconfig allows you to create a scaffolding in your repository to configure what Zuul should do on specific git events:

* The pipelines to trigger
* The jobs to run

The command

```sh
./tools/sfconfig bootstrap-tenant-config-repo --connection [connection] --outpath [/path/to/repository]
```

will create, if needed:

* the `zuul.d` folder, with template `jobs.yaml` and `pipeline.yaml` files
* the `playbooks` folder, where Ansible playbooks defining your jobs can be defined.

### Modify and merge

The scaffolding is yours to modify to suit your needs. Once you are happy with your changes, commit them and push them to your code review system.
If all went well and provided you configured a check pipeline or equivalent, you should see your modifications trigger events on Zuul.