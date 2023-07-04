#!/bin/sh

set -ex

export HOME=/var/lib/zuul

# Generate the default tenants configuration file
cat << EOF > ~/main.yaml
- tenant:
    max-job-timeout: 10800
    name: internal
    report-build-page: true
    source:
      git-server:
        config-projects:
        - system-config
      opendev.org:
        untrusted-projects:
        - zuul/zuul-jobs
EOF

if [ "$CONFIG_REPO_SET" == "TRUE" ]; then
  # A config repository has been set

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

  # Ensure the config repo enabled into the tenants config
  cat << EOF >> ~/main.yaml
      ${CONFIG_REPO_CONNECTION_NAME}:
        config-projects:
        - ${CONFIG_REPO_NAME}
EOF

  # Append the config repo provided tenant file to the default one
  if [ -f ~/${CONFIG_REPO_NAME}/zuul/main.yaml ]; then
    cat ~/${CONFIG_REPO_NAME}/zuul/main.yaml >> ~/main.yaml
  fi

fi