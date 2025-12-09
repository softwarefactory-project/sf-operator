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

Make also sure that any specific upgrade instructions are mentioned in the [upgrades guidelines](../deployment/upgrades.md) if
needed.

Then commit the changes for review, and have them validated and merged. **This change should be the last one before tagging.**

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

The release process is currently manual. This section will be updated once the automation is in place.