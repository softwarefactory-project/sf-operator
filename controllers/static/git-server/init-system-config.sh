#!/bin/bash

set -ex

env

[ ! -d /git/system-config ] && git init --bare /git/system-config

cd ${HOME}
cat << EOF > .gitconfig
[user]
    name = SF initial configurator
    email = admin@${FQDN}
[gitreview]
    username = admin
[push]
    default = simple
EOF

cd /tmp
[ -d /tmp/system-config ] && rm -Rf /tmp/system-config
git clone /git/system-config
cd /tmp/system-config

mkdir -p zuul.d playbooks/base playbooks/config

cat << EOF > zuul.d/jobs-base.yaml
- job:
    name: base
    parent: null
    description: The base job.
    pre-run: playbooks/base/pre.yaml
    post-run:
      - playbooks/base/post.yaml
    timeout: 1800
    attempts: 3

- semaphore:
    name: semaphore-config-update
    max: 1

- job:
    name: config-check
    parent: base
    final: true
    description: Validate the config repo.
    run: playbooks/config/check.yaml
    nodeset:
      nodes: []

- job:
    name: config-update
    parent: base
    final: true
    description: Deploy config repo update.
    run: playbooks/config/update.yaml
    semaphore: semaphore-config-update
    nodeset:
      nodes: []
EOF

cat << EOF > playbooks/base/pre.yaml
- hosts: localhost
  tasks: []
EOF

cat << EOF > playbooks/base/post.yaml
- hosts: localhost
  tasks: []
EOF

cat << EOF > playbooks/config/check.yaml
- hosts: localhost
  tasks: []
EOF

cat << EOF > playbooks/config/update.yaml
- hosts: localhost
  tasks: []
EOF

git add zuul.d playbooks
git commit -m"Set system config base jobs" && git push origin master || true