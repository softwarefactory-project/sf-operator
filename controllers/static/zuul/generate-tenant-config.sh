#!/bin/sh

set -ex
REF=$1
REF=${REF:-origin/master}

if [ "${HOME}" == "/" ]; then
    echo "HOME can not be / dir!"
    exit 1
fi

export GIT_SSH_COMMAND="ssh -i /var/lib/admin-ssh/..data/priv -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

if [ -d "${HOME}/config/.git" ]; then
  pushd "${HOME}/config"
  git fetch origin
  git reset --hard $REF
  popd
else
  pushd "${HOME}"
  git clone ssh://${CONFIG_REPO_USER}@${CONFIG_REPO_URL}
  popd
fi

managesf-configuration --gateway-url "https://${FQDN}" \
  --cache-dir $HOME \
  --config-dir $HOME/config \
  --default-tenant-name internal \
  --output $HOME/main.yaml zuul
