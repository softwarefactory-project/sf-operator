#!/bin/sh

set -ex

export HOME=/var/lib/zuul

if [ "${ZUUL_STARTUP}" == "true" -a ! -f /var/lib/zuul/main.yaml ] || [ "${ZUUL_STARTUP}" == "false" ]; then

  REF=$1
  REF=${REF:-origin/master}

  # Clone or fetch config repository
  if [ -d ~/${CONFIG_REPO_NAME}/.git ]; then
    pushd ~/${CONFIG_REPO_NAME}
    git remote remove origin
    git remote add origin ${CONFIG_REPO_BASE_URL}/${CONFIG_REPO_NAME}
    git fetch origin
    git reset --hard $REF
    popd
  else
    pushd ~/
    git clone ${CONFIG_REPO_BASE_URL}/${CONFIG_REPO_NAME}
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
      ${CONFIG_REPO_CONNECTION_NAME}:
        config-projects:
        - ${CONFIG_REPO_NAME}
      git-server:
        config-projects:
        - system-config
      opendev.org:
        untrusted-projects:
        - zuul/zuul-jobs
EOF

if [ -f ~/${CONFIG_REPO_NAME}/zuul/main.yaml ]; then
  cat ~/${CONFIG_REPO_NAME}/zuul/main.yaml >> ~/main.yaml
fi

else
  echo "Conditions to run the tenant config generation did not match."
fi