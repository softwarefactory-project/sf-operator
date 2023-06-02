#!/bin/bash

# This script creates a new job definition in Zuul CI, then it makes
# a PatchSet in Gerrit to trigger the new job.

COMMAND=${COMMAND:-"sleep 300"}
SF_OPERATOR_DIR=$(readlink -f "$(pwd)")
GERRIT_ADMIN_API_KEY=$("${SF_OPERATOR_DIR}/tools/get-secret.sh" gerrit-admin-api-key)

if ! command -v pip3; then
    echo "Please install pip package!"
    exit 1
fi

if ! command -v git-review; then
    python3 -mvenv .venv && .venv/bin/pip3 install git-review
    VENV_PATH=$PATH:$(pwd)/.venv/bin
    export PATH=$VENV_PATH
fi

git clone \
    -c user.name="Admin" \
    -c user.email="admin@sftests.com" \
    -c http.sslVerify=false \
    "https://admin:${GERRIT_ADMIN_API_KEY}@gerrit.sftests.com/a/config" /tmp/config && \
    cd /tmp/config

mkdir -p playbooks

git config user.name "Admin"
git config user.email admin@sftests.com
git config http.sslVerify false

git remote add gerrit "https://admin:${GERRIT_ADMIN_API_KEY}@gerrit.sftests.com/a/config"

cat << EOF > zuul.d/config.yaml
---
- job:
    name: test
    run: playbooks/test.yml

- project:
    check:
      jobs:
        - config-check
        - test
    gate:
      jobs:
        - config-check
    post:
      jobs:
        - config-update
EOF

cat << EOF > playbooks/test.yml
---
- hosts: localhost,all
  tasks:
    - name: Doing command
      command: $COMMAND
EOF

git add .
git commit -m "Add check job"
git push origin master


echo "test" > test-file
git add test-file
git commit -m 'Executing test PS'
git-review

cd -
