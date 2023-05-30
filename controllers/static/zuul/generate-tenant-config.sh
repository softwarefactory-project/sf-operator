#!/bin/sh

set -ex

if [ "${HOME}" != "/var/lib/zuul" ]; then
    echo "HOME must be /var/lib/zuul !"
    exit 1
fi

if [ "${ZUUL_STARTUP}" == "true" -a ! -f /var/lib/zuul/main.yaml ] || [ "${ZUUL_STARTUP}" == "false" ]; then

  REF=$1
  REF=${REF:-origin/master}

  export GIT_SSH_COMMAND="ssh -i /var/lib/admin-ssh/..data/priv -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

  # Clone or fetch config repository
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

  # Generate Zuul tenant config
  # The HOME is a shared volume with the zuul-scheduler pod
  managesf-configuration --gateway-url "https://${FQDN}" \
    --cache-dir $HOME \
    --config-dir $HOME/config \
    --default-tenant-name internal \
    --output $HOME/main.yaml zuul
else
  echo "Conditions to run the tenant config generation did not match."
fi
