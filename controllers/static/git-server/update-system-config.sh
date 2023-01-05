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

- job:
    name: sleeper
    run: playbooks/sleeper.yaml

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

# TODO: setup allowed-project rules
- job:
    name: config-update
    parent: base
    final: true
    description: Deploy config repo update.
    run: playbooks/config/update.yaml
    semaphore: semaphore-config-update
    secrets:
      - k8s_config
    nodeset:
      nodes: []

# TODO: decide where the pipeline should live, e.g. system-config or user config,
# and how to add the zuul connections.
- pipeline:
    name: post
    post-review: true
    description: This pipeline runs jobs that operate after each change is merged.
    manager: supercedent
    precedence: low
    trigger:
      git-server:
        event:
          - ref-updated
      gerrit:
        - event: ref-updated
          ref: ^refs/heads/.*$

# TODO: hardcode for now on the internal Gerrit but must be configured based
# on the CRD (for the config repo location).
- pipeline:
    name: check
    description: |
      Newly uploaded patchsets enter this pipeline to receive an
      initial +/-1 Verified vote.
    manager: independent
    require:
      gerrit:
        open: True
        current-patchset: True
    trigger:
      gerrit:
        - event: patchset-created
        - event: change-restored
        - event: comment-added
          comment: (?i)^(Patch Set [0-9]+:)?( [\w\\+-]*)*(\n\n)?\s*(recheck|reverify)
        - event: comment-added
          require-approval:
            - Verified: [-1, -2]
              username: zuul
          approval:
            - Workflow: 1
    start:
      gerrit:
        Verified: 0
    success:
      gerrit:
        Verified: 1
    failure:
      gerrit:
        Verified: -1

- pipeline:
    name: gate
    description: |
      Changes that have been approved by core developers are enqueued
      in order in this pipeline, and if they pass tests, will be
      merged.
    success-message: Build succeeded (gate pipeline).
    failure-message: Build failed (gate pipeline). 
    manager: dependent
    precedence: high
    supercedes: check
    post-review: True
    require:
      gerrit:
        open: True
        current-patchset: True
        approval:
          - Workflow: 1
    trigger:
      gerrit:
        - event: comment-added
          approval:
            - Workflow: 1
        - event: comment-added
          approval:
            - Verified: 1
          username: zuul
    start:
      gerrit:
        Verified: 0
    success:
      gerrit:
        Verified: 2
        submit: true
    failure:
      gerrit:
        Verified: -2
    window-floor: 20
    window-increase-factor: 2

- project:
    post:
      jobs:
        - sleeper
EOF


cat << EOF > playbooks/sleeper.yaml
- hosts: localhost
  tasks:
    - debug:
        msg: "Hello zuul, i'm taking a nap"

    - command: sleep 600
EOF

cat << EOF > playbooks/base/pre.yaml
- hosts: localhost
  tasks:
    - debug:
        var: zuul

- hosts: all
  tasks:
    - zuul_console:
EOF

cat << EOF > playbooks/base/post.yaml
- hosts: localhost
  tasks: []
EOF

cat << EOF > playbooks/config/check.yaml
- hosts: localhost
  tasks:
    - name: Set speculative config path
      set_fact:
        config_root: "{{ zuul.executor.src_root }}/{{ zuul.project.canonical_name }}"

    - name: "Access config repo"
      command: "ls -al ./resources"
      args:
        chdir: '{{ config_root }}'
EOF

cat << EOF > playbooks/config/update.yaml
- hosts: localhost
  roles:
    - setup-k8s-config
    - apply-k8s-resource
EOF

mkdir -p roles/setup-k8s-config/tasks
cat << EOF > roles/setup-k8s-config/tasks/main.yaml
- name: ensure config dir
  file:
    path: "{{ ansible_env.HOME }}/.kube"
    state: directory

- name: copy secret content
  copy:
    content: "{{ k8s_config['ca.crt'] }}"
    dest: "{{ ansible_env.HOME }}/.kube/ca.crt"
    mode: "0600"

- name: setup config
  command: "{{ item }}"
  no_log: true
  loop:
    - "kubectl config set-cluster local --server='{{ k8s_config['server'] }}' --certificate-authority={{ ansible_env.HOME }}/.kube/ca.crt"
    - "kubectl config set-credentials local-token --token={{ k8s_config['token'] }}"
    - "kubectl config set-context local-context --cluster=local --user=local-token --namespace={{ k8s_config['namespace'] }}"
    - "kubectl config use-context local-context"
EOF

mkdir -p roles/apply-k8s-resource/tasks
cat << EOF > roles/apply-k8s-resource/tasks/main.yaml
- name: Display available resources
  command: "kubectl api-resources"
#- name: ensure system config is up-to-date
#  command: "kubectl apply -f {{ zuul.project.src_dir }}/system/sf.yaml"
EOF

git add zuul.d playbooks roles

if [ ! -z "$(git status --porcelain)" ]; then
  git commit -m"Set system config base jobs"
  git push origin master
fi