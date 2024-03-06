This page documents the release process of the SF operator.

# CHANGELOG Management

We follow the guidelines of [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) to manage our CHANGELOG.

Before tagging, please make sure the [CHANGELOG](../reference/CHANGELOG.md) is up to date:

1. Rename the `[in development]` section to the next tag, followed by the UTC date as `YYYY-MM-DD` of the tag (for example `[v0.0.20] - 2023-12-31`)
1. Remove any empty sections in the release block
1. Prepend a template `[in development]` section like so:

```markdown
## [in development]

### Added
### Changed
### Deprecated
### Removed
### Fixed
### Security
```

Then commit the changes to the CHANGELOG for review, and have them validated and merged. **This change should be the last one before tagging.**

# Tagging

!!! note
    Only core contributors have the right to push a tag on the `sf-operator` repository.
    If you aren't a core contributor but would like to suggest the creation of a new tag,
    please contact us on [our matrix channel](https://matrix.to/#/#softwarefactory-project:matrix.org).

Here are the commands to run (assuming releasing HEAD to v0.0.20 version):

```sh
git checkout master
git pull origin master
git tag v0.0.20 HEAD
git push gerrit v0.0.20
```


# Release Automation

A CD job named [sf-operator-publish-olm-bundle-image](https://microshift.softwarefactory-project.io/zuul/t/local/builds?job_name=sf-operator-publish-olm-bundle-image&skip=0) runs in the `release` pipeline.
The `release` pipeline is triggered when a git tag is created on the `sf-operator` repository.
This job builds and pushes the following assets to Quay.io:

- [A bundle image](https://quay.io/repository/software-factory/sf-operator-bundle?tab=tags)
- [A catalog image](https://quay.io/repository/software-factory/sf-operator-catalog?tab=tags)
- [An operator image](https://quay.io/repository/software-factory/sf-operator?tab=tags)

Clusters where the operator is installed through OLM will upgrade automatically to the latest released version.