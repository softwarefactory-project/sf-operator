#!/bin/bash

# This script creates a new job definition in Zuul CI, then it makes
# a PatchSet in Gerrit to trigger the new job.

COMMAND=${COMMAND:-"sleep 300"}
SF_OPERATOR_DIR=$(readlink -f "$(pwd)")
GERRIT_ADMIN_API_KEY=$("${SF_OPERATOR_DIR}/tools/get-secret.sh" gerrit-admin-api-key)
TMP_DIR=$(mktemp -d)

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
    -c user.email="admin@sfop.me" \
    -c http.sslVerify=false \
    "https://admin:${GERRIT_ADMIN_API_KEY}@gerrit.sfop.me/a/config" "$TMP_DIR/config"

cd "$TMP_DIR/config"
mkdir -p "$TMP_DIR/config/playbooks"
mkdir -p "$TMP_DIR/config/zuul.d"

git config user.name "Admin"
git config user.email admin@sfop.me
git config http.sslVerify false
git remote add gerrit "https://admin:${GERRIT_ADMIN_API_KEY}@gerrit.sfop.me/a/config"

cat << EOF > zuul.d/config.yaml
---
- job:
    name: test-job
    run: playbooks/test.yml
    cleanup-run:
      name: playbooks/sleep.yml
    nodeset:
      nodes:
        - name: container
          label: zuul-worker-sf-operator-ci

- project:
    check:
      jobs:
        - test-job
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

cat << EOF > playbooks/sleep.yml
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

echo -e "\n\nTemp dir is: $TMP_DIR\n\n"
cd -
