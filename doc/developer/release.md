# How to release the sf-operator

A CI job named [sf-operator-publish-olm-bundle-image](https://zuul.microshift.softwarefactory-project.io/zuul/t/local/builds?job_name=sf-operator-publish-olm-bundle-image&skip=0) runs in the `release` pipeline.
The `release` pipeline is triggered when a git tag is created on the `sf-operator` repository.
This job builds and pushes the following assets to Quay.io:

- [A bundle image](https://quay.io/repository/software-factory/sf-operator-bundle?tab=tags)
- [A catalog image](https://quay.io/repository/software-factory/sf-operator-catalog?tab=tags)
- [An operator image](https://quay.io/repository/software-factory/sf-operator?tab=tags)

## Tagging sf-operator

> Only core contributors have the right to push a tag on the `sf-operator` repository.
If you aren't a core contributor but would like to suggest the creation of a new tag,
please contact us on [our matrix channel](https://matrix.to/#/#softwarefactory-project:matrix.org).

Here are the commands to run (assuming releasing HEAD to 0.0.6 version):

```sh
git tag 0.0.6 HEAD
git push gerrit 0.0.6
```
