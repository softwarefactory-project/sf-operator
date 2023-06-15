#!/bin/bash

set -ex

env

[ ! -d /git/system-config ] && git init --bare /git/system-config

cd /tmp
[ -d /tmp/system-config ] && rm -Rf /tmp/system-config
git clone /git/system-config
cd /tmp/system-config

git config user.name "sf-operator"
git config user.email "admin@${FQDN}"

mkdir -p zuul.d playbooks/base playbooks/config

cat << EOF > zuul.d/jobs-base.yaml
- job:
    name: base
    parent: null
    description: The base job.
    pre-run: playbooks/base/pre.yaml
    post-run:
      - playbooks/base/post.yaml
    roles:
      - zuul: zuul/zuul-jobs
    timeout: 1800
    attempts: 3
    secrets:
      - site_sflogs

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

- project:
    post:
      jobs:
        - sleeper
EOF

if [ -n "${CONFIG_REPO_NAME}" -a -n "${CONFIG_ZUUL_CONNECTION_NAME}" ]; then
  cat << EOF > zuul.d/config-project.yaml
---
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
      ${CONFIG_ZUUL_CONNECTION_NAME}:
        - event: ref-updated
          ref: ^refs/heads/.*$

- pipeline:
    name: check
    description: |
      Newly uploaded patchsets enter this pipeline to receive an
      initial +/-1 Verified vote.
    manager: independent
    require:
      ${CONFIG_ZUUL_CONNECTION_NAME}:
        open: True
        current-patchset: True
    trigger:
      ${CONFIG_ZUUL_CONNECTION_NAME}:
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
      ${CONFIG_ZUUL_CONNECTION_NAME}:
        Verified: 0
    success:
      ${CONFIG_ZUUL_CONNECTION_NAME}:
        Verified: 1
    failure:
      ${CONFIG_ZUUL_CONNECTION_NAME}:
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
      ${CONFIG_ZUUL_CONNECTION_NAME}:
        open: True
        current-patchset: True
        approval:
          - Workflow: 1
    trigger:
      ${CONFIG_ZUUL_CONNECTION_NAME}:
        - event: comment-added
          approval:
            - Workflow: 1
        - event: comment-added
          approval:
            - Verified: 1
          username: zuul
    start:
      ${CONFIG_ZUUL_CONNECTION_NAME}:
        Verified: 0
    success:
      ${CONFIG_ZUUL_CONNECTION_NAME}:
        Verified: 2
        submit: true
    failure:
      ${CONFIG_ZUUL_CONNECTION_NAME}:
        Verified: -2
    window-floor: 20
    window-increase-factor: 2

- project:
    name: ${CONFIG_REPO_NAME}
    check:
      jobs:
        - config-check
    gate:
      jobs:
        - config-check
    post:
      jobs:
        - config-update
EOF
else
  echo "---" > zuul.d/config-project.yaml
fi

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
    - block:
        - import_role:
            name: emit-job-header
        - import_role:
            name: log-inventory
      vars:
        zuul_log_url: "https://logserver.${FQDN}/"

- hosts: all
  tasks:
    - include_role: name=start-zuul-console
    - block:
        - include_role: name=validate-host
        - include_role: name=prepare-workspace
        - include_role: name=add-build-sshkey
      when: "ansible_connection != 'kubectl'"
    - block:
        - include_role: name=prepare-workspace-openshift
        - include_role: name=remove-zuul-sshkey
      run_once: true
      when: "ansible_connection == 'kubectl'"
    - import_role: name=ensure-output-dirs
      when: ansible_user_dir is defined
EOF

cat << EOF > playbooks/base/post.yaml
- hosts: localhost
  roles:
    -  role: add-fileserver
       fileserver: "{{ site_sflogs }}"
    -  role: generate-zuul-manifest

- hosts: logserver-sshd
  vars:
    ansible_port: ${LOGSERVER_SSHD_SERVICE_PORT}
  gather_facts: false
  tasks:
    - block:
        - import_role:
            name: upload-logs
        - import_role:
            name: buildset-artifacts-location
      vars:
        zuul_log_compress: true
        zuul_log_url: "https://logserver.${FQDN}/"
        zuul_logserver_root: "{{ site_sflogs.path }}"
        zuul_log_verbose: true
EOF

cat << EOF > playbooks/config/check.yaml
- hosts: localhost
  tasks:
    - name: Set speculative config path
      set_fact:
        config_root: "{{ zuul.executor.src_root }}/{{ zuul.project.canonical_name }}"

    - name: "Access config repo"
      command: "ls -al ./"
      args:
        chdir: '{{ config_root }}'
EOF

cat << EOF > playbooks/config/update.yaml
- hosts: localhost
  roles:
    - setup-k8s-config
    - add-k8s-hosts

- hosts: zuul-scheduler-sidecar
  vars:
    config_ref: "{{ zuul.newrev | default('origin/master') }}"
  tasks:
    - name: "Update zuul tenant config"
      command: /usr/local/bin/generate-zuul-tenant-yaml.sh "{{ config_ref }}"

- hosts: zuul-scheduler
  tasks:
    - name: "Reconfigure the scheduler"
      command: zuul-scheduler full-reconfigure

- hosts: nodepool-launcher-sidecar
  vars:
    config_ref: "{{ zuul.newrev | default('origin/master') }}"
    ansible_remote_tmp: "/tmp/ansible/.tmp"
  tasks:
    - name: "Update nodepool-launcher config"
      command: /usr/local/bin/generate-launcher-config.sh "{{ config_ref }}"

EOF

mkdir -p roles/add-k8s-hosts/tasks
cat << EOF > roles/add-k8s-hosts/tasks/main.yaml
- ansible.builtin.add_host:
    name: "zuul-scheduler-sidecar"
    ansible_connection: kubectl
    # https://docs.ansible.com/ansible/latest/collections/kubernetes/core/kubectl_connection.html#ansible-collections-kubernetes-core-kubectl-connection
    ansible_kubectl_container: scheduler-sidecar
    ansible_kubectl_pod: "zuul-scheduler-0"

- ansible.builtin.add_host:
    name: "zuul-scheduler"
    ansible_connection: kubectl
    ansible_kubectl_container: zuul-scheduler
    ansible_kubectl_pod: "zuul-scheduler-0"

- name: Fetch nodepool-launcher Pod info
  # https://docs.ansible.com/ansible/latest/collections/kubernetes/core/k8s_info_module.html
  kubernetes.core.k8s_info:
    kind: Pod
    label_selectors:
      - "run = nodepool-launcher"
    namespace: "{{ k8s_config['namespace'] }}"
  register: nodepool_launcher_info

- ansible.builtin.add_host:
    name: "nodepool-launcher-sidecar"
    ansible_connection: kubectl
    ansible_kubectl_pod: "{{ nodepool_launcher_info.resources[0].metadata.name }}"
    ansible_kubectl_container: nodepool-launcher-sidecar

EOF

mkdir -p roles/setup-k8s-config/tasks
cat << EOF > roles/setup-k8s-config/tasks/main.yaml
- name: ensure config dir
  file:
    path: "{{ ansible_env.HOME }}/.kube"
    state: directory

- name: copy secret content
  ansible.builtin.copy:
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

git add zuul.d playbooks roles

if [ ! -z "$(git status --porcelain)" ]; then
  git commit -m"Set system config base jobs"
  git push origin master
fi
