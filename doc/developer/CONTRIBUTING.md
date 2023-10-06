# Contributing

We welcome all contributions to the project!

General guidelines about contributing to the SF-Operator can be found in this document.
For further details about the code base, testing and hacking the project, please see
the [Developer documentation](doc/developer/index.md).

## Project repositories

The main repository of the project is hosted at [softwarefactory-project.io](https://softwarefactory-project.io/r/software-factory/sf-operator).

The custom container images used by the SF-Operator are defined in the [container-pipeline project](https://softwarefactory-project.io/r/containers) and
published on [quay.io](https://quay.io/organization/software-factory).

Use the [git-review workflow](https://softwarefactory-project.io/docs/user/contribute.html#create-a-new-code-review) to interact with these projects.

Repositories with the same name on GitHub are mirrors from **softwarefactory-project.io**, no pull request will be accepted there.

The MicroShift deployment Ansible role is hosted on [GitHub](https://github.com/openstack-k8s-operators/ansible-microshift-role). Pull Requests are welcome there.

## Architectural Decision Records (ADRs)

Any large contribution aiming to modify or implement a functionality must be first validated by the community with
an *[Architectural Decision Record](https://adr.github.io/) (ADR)*.

ADRs can be created following the [template found in the docs/adr](doc/adr/adr-template.md) directory of
the sf-operator repository.

## Review Checklist

Before submitting a change or a patch chain for review, please consider the following checklist:

1. Are the commit messages clear and explanatory?
1. Do the changes need to be documented in the changelog?
1. Do the changes cover any required modification of the existing documentation?
1. Are the changes tested? We do not require unit testing but do expect functional testing coverage.
