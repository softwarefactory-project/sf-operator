#!/bin/sh

set -ex

REF=$1
REF=${REF:-origin/master}

export GIT_SSH_COMMAND="ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

if [ -d "$HOME/config/.git" ]; then
  pushd $HOME/config
  git fetch origin
  git reset --hard $REF
  popd
else
  pushd $HOME
  git clone ssh://${CONFIG_REPO_USER}@${CONFIG_REPO_URL}
  popd
fi

managesf-configuration --gateway-url "https://${FQDN}" \
  --config-dir $HOME/config \
  --default-tenant-name internal \
  --output $HOME/main.yaml zuul

cp -f $HOME/main.yaml /var/lib/zuul/main.yaml