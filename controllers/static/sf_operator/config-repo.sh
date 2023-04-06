#!/bin/bash

# This script is the entry point to populate a SF config repository
# according the config-locations spec. For the moment that's a shell script
# but we can rewrite it into python to make it a bit cleaner.

set -ex

env
cd ${HOME}

cat /etc/hosts

mkdir .ssh
chmod 0700 .ssh
echo "${SF_ADMIN_SSH}" > .ssh/id_rsa
chmod 0400 .ssh/id_rsa

cat << EOF > .gitconfig
[user]
    name = SF initial configurator
    email = admin@${FQDN}
[push]
    default = simple
EOF

export GIT_SSH_COMMAND="ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

git clone ssh://${CONFIG_REPO_USER}@${CONFIG_REPO_URL}

cd config

# Initialize resources tree
if [ ! -d resources ]; then
  mkdir resources
  dhall-to-yaml --output \
    resources/resources.yaml <<< "(/sf_operator/resources.dhall).renderEmptyResources"
fi

# Update the _internal.yaml resources file
dhall-to-yaml --output \
  resources/_internal.yaml <<< "(/sf_operator/resources.dhall).renderInternalResources \"${FQDN}\" True"

# Initialize zuul directory
if [ ! -d zuul ]; then
  mkdir zuul
  touch zuul/jobs.yaml
fi

# Initialize system resource
mkdir -p system
cat /sf-provided-cr/sf.yaml > system/sf.yaml

# Initialize gerrit tree
if [ ! -d gerrit ] && [ "${GERRIT_ENABLED}" == "true" ]; then
  mkdir gerrit && pushd gerrit
  cat << EOF > replication.config
[gerrit]
    defaultForceUpdate = true
    replicateOnStartup = true
    autoReload = true
EOF
  cat << EOF > commentlinks.yaml
---
# Note: '\' character needs to be escapped twice ('\\')
# Double quote needs to be escaped too
commentlinks: []
EOF
  popd
fi

# Initialize CI
mkdir -p zuul.d
cat << EOF > zuul.d/config.yaml
- project:
    check:
      jobs:
        - config-check
    gate:
      jobs:
        - config-check
    post:
      jobs:
        - config-update
EOF

git add -A
git commit -m"Populate config repository" && git push origin master || true
