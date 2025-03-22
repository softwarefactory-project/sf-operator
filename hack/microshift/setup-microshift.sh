#!/bin/bash
# Copyright © 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

# This script handles the installation of MicroShift (OpenShift) on a RHEL Machine
# It also performs some preparation to welcome a Software Factory deployment

fp=$(readlink -f -- "${BASH_SOURCE[0]}")
dp=$(dirname "${fp}")
logs=~/.cache/setup-microshift.log

fail() {
    echo "ERROR: $*" >&2
    exit 1
}

run() {
    echo -n "⏳ Running $@ ... "
    "$@" >> $logs 2>&1
    if [ $? -eq 0 ]; then
        echo "✅"
    else
        echo "❌ Failed (check logs into $dp/setup-microshift.log)"
        cat $logs
        exit 1
    fi
}

if ! [ $# -ge 1 ]; then
    cat << EOF

Usage: $0 [host] [user]
Example: $0 localhost
         $0 my-host-ip-address cloud-user
EOF
    fail "Missing host parameter"
fi

ANSIBLE_HOST="$1"
ANSIBLE_USER="${ANSIBLE_USER:-$2}"
PULL_SECRET_FILE_PATH="${PULL_SECRET_FILE_PATH:-$(realpath ~/openshift-pull-secret.yaml)}"
ANSIBLE_CONFIG_PATH=$dp/ansible.cfg
ANSIBLE_MICROSHIFT_ROLE_PATH=~/src/github.com/openstack-k8s-operators/ansible-microshift-role
ANSIBLE_MICROSHIFT_ROLE_HASH=364d85e856a264724ee213bdc19d8aaa3815cb7f

ensure_pull_secret() {
  # Check if pull secret file exists (microshift requires a pull secret file)
  if [ ! -f "$PULL_SECRET_FILE_PATH" ]; then
    fail "Pull secret file not found: $PULL_SECRET_FILE_PATH"
  fi
}

if [ -z "$ANSIBLE_USER" ]; then
    ANSIBLE_USER="zuul-worker"
    echo "No ANSIBLE_USER set, using 'zuul-worker'"
fi

ensure_basic_tools() {
  echo "Ensuring basic toolings installed"
  $dp/../setup-ansible.sh
  if ! command -v git >/dev/null 2>&1; then
      sudo dnf install -y git
  fi
}

ensure_ansible_galaxy_collections() {
  echo "Ensure the installation the ansible collections"
  # NOTE: Workaround issues when ansible-galaxy silently fail to install a collection
  for i in $(seq 10); do
      INSTALLED=$(ansible-galaxy collection list)
      (echo $INSTALLED | grep -q community.general) && (echo $INSTALLED | grep -q community.crypto) && (echo $INSTALLED | grep -q ansible.posix) && break
      ansible-galaxy collection install --timeout=15 -vv \
          git+https://github.com/ansible-collections/community.general \
          git+https://github.com/ansible-collections/community.crypto \
          git+https://github.com/ansible-collections/ansible.posix || echo "Failed to install collection: $INSTALLED"
  done
}

ensure_ansible_inventory() {
    echo "Preparing the ansible inventory file"
    if [ "$ANSIBLE_HOST" == "localhost" ]; then
        ANSIBLE_CONNECTION="local"
    else
        ANSIBLE_CONNECTION="ssh"
    fi
    cat << EOF > ~/.cache/inventory.yaml
---
all:
  hosts:
    controller:
      ansible_user: "$ANSIBLE_USER"
      ansible_host: "$ANSIBLE_HOST"
      ansible_connection: "$ANSIBLE_CONNECTION"
EOF
}

ensure_microshift_ansible_role() {
  echo "Ensuring a checkout of ansible-microshift-role"
  mkdir -p $(dirname $ANSIBLE_MICROSHIFT_ROLE_PATH)
  if [ ! -d $ANSIBLE_MICROSHIFT_ROLE_PATH/.git ]; then
      pushd $(dirname $ANSIBLE_MICROSHIFT_ROLE_PATH)
      git clone https://github.com/openstack-k8s-operators/ansible-microshift-role
      popd
  else
      git -C $ANSIBLE_MICROSHIFT_ROLE_PATH fetch origin
  fi
  git -C $ANSIBLE_MICROSHIFT_ROLE_PATH reset --hard $ANSIBLE_MICROSHIFT_ROLE_HASH
}

ensure_microshift() {
  echo "Deploying and configuring MicroShift on host $ANSIBLE_USER@$ANSIBLE_HOST..."
  ansible-playbook \
      -i ~/.cache/inventory.yaml \
      -e "@$dp/vars.yaml" \
      -e "@${PULL_SECRET_FILE_PATH}" \
      -e "microshift_role_path=$ANSIBLE_MICROSHIFT_ROLE_PATH" \
      $dp/do-microshift.yaml
}

ensure_local_kubeconfig() {
  echo "Ensure a local ~/.kube/microshift-config to access the deployment"
  mkdir -p ~/.kube
  ansible controller \
      -i ~/.cache/inventory.yaml \
      -m "ansible.builtin.fetch" \
      -a '{"src": "~/.kube/config", "dest":"~/.kube/microshift-config", "flat":"true"}'
}

export ANSIBLE_CONFIG=$ANSIBLE_CONFIG_PATH

mkdir -p $(dirname $logs)
echo "" > $logs

echo -e "ℹ️  This command logs into $logs"
echo ""
echo "▶️  == Running preparation steps =="
run ensure_pull_secret
run ensure_basic_tools
run ensure_ansible_galaxy_collections
run ensure_ansible_inventory
run ensure_microshift_ansible_role
echo ""
echo "▶️  == Deploying MicroShift on $ANSIBLE_USER@$ANSIBLE_HOST (~5 minutes) =="
run ensure_microshift
run ensure_local_kubeconfig
echo ""
echo -e "🚀 Deploying Microshift done 🚀"
echo ""
echo "To access the deployment, run: KUBECONFIG=~/.kube/microshift-config kubectl -n sf get pods"
