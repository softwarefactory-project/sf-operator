# Contributing

We welcome all contributions to the project!

General guidelines about contributing to the SF-Operator can be found in this document.

## Project repositories

The main repository of the project is hosted at [softwarefactory-project.io](https://softwarefactory-project.io/r/plugins/gitiles/software-factory/sf-operator).

The custom container images used by the SF-Operator are defined in the [container-pipeline project](https://softwarefactory-project.io/r/containers) and
published on [quay.io](https://quay.io/organization/software-factory).

Use the [git-review workflow](https://softwarefactory-project.io/docs/user/contribute.html#create-a-new-code-review) to interact with these projects.

Repositories with the same name on GitHub are mirrors from **softwarefactory-project.io**, and no pull requests will be accepted there.

The MicroShift deployment Ansible role is hosted on [GitHub](https://github.com/openstack-k8s-operators/ansible-microshift-role). Pull Requests are welcome there.

## Architectural Decision Records (ADRs)

Any large contribution aiming to modify or implement a functionality must first be validated by the community with
an *[Architectural Decision Record](https://adr.github.io/) (ADR)*.

For new ADRs, please use the template below as a boilerplate:

??? abstract "ADR template"

    ```markdown title="template.md"
    --8<-- "doc/reference/adr/adr-template.md"
    ```

## Review Checklist

Before submitting a change or a patch chain for review, please consider the following checklist:

1. Are the commit messages clear and explanatory?
1. Do the changes need to be documented in the changelog?
1. Do the changes cover any required modifications of the existing documentation? (see [guidelines](#documentation-guidelines) below)
1. Are the changes tested? We do not require unit testing but do expect functional testing coverage.

## Documentation guidelines

The documentation is written in Markdown, as implemented by GitHub Pages. Please refer to
[this documentation](https://www.markdownguide.org/tools/github-pages/) to check what elements are supported.

Any change that implements a new feature or significantly changes an existing one must be reflected
in the documentation, in the impacted section(s).

### API Documentation

The API documentation is auto-generated with [crd-ref-docs](https://github.com/elastic/crd-ref-docs).

Running `make` or `make build` or `make build-api-doc` will update the API documentation if needed.

### CLI Documentation

For now, the CLI documentation must be updated by hand.
