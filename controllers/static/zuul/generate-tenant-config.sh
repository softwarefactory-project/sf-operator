#!/bin/sh

set -ex

export HOME=/var/lib/zuul

if [ "${ZUUL_STARTUP}" == "true" -a ! -f /var/lib/zuul/main.yaml ] || [ "${ZUUL_STARTUP}" == "false" ]; then

  REF=$1
  REF=${REF:-origin/master}

  export GIT_SSH_COMMAND="ssh -i /var/lib/admin-ssh/..data/priv -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

  # Clone or fetch config repository
  if [ -d ~/config/.git ]; then
    pushd ~/config
    git fetch origin
    git reset --hard $REF
    popd
  else
    pushd ~/
    git clone ssh://${CONFIG_REPO_USER}@${CONFIG_REPO_URL}
    popd
  fi

  cat << EOF > ~/main.yaml
- authorization-rule:
    conditions:
    - preferred_username: admin
    - roles: zuul_admin
    name: __SF_DEFAULT_ADMIN
- authorization-rule:
    conditions:
    - roles: '{tenant.name}_zuul_admin'
    name: __SF_TENANT_ZUUL_ADMIN
- tenant:
    admin-rules:
    - __SF_DEFAULT_ADMIN
    - __SF_TENANT_ZUUL_ADMIN
    max-job-timeout: 10800
    name: internal
    report-build-page: true
    source:
      gerrit:
        config-projects:
        - config: {}
      git-server:
        config-projects:
        - system-config: {}
      opendev.org:
        untrusted-projects:
        - zuul/zuul-jobs: {}
EOF

if [ -f ~/config/zuul/main.yaml ]; then
  cat ~/config/zuul/main.yaml >> ~/main.yaml
fi

else
  echo "Conditions to run the tenant config generation did not match."
fi