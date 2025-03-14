#!/bin/sh
# Copyright © 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

fail() {
    echo "ERROR: $*" >&2
    exit 1
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
ANSIBLE_CONFIG=${ANSIBLE_CONFIG:-''}

# Check if pull secret file exists
if [ ! -f "$PULL_SECRET_FILE_PATH" ]; then
    fail "Pull secret file not found: $PULL_SECRET_FILE_PATH"
fi

if [ -z "$ANSIBLE_USER" ]; then
    ANSIBLE_USER="zuul-worker"
    echo "No ANSIBLE_USER set, using 'zuul-worker'"
fi

if [ -z "$ANSIBLE_CONFIG" ]; then
    echo "No ansible config found, exporting ANSIBLE_CONFIG"
    export ANSIBLE_CONFIG="$(realpath tools/microshift/ansible.cfg)"
fi

./tools/setup-ansible.sh

if ! command -v git >/dev/null 2>&1; then
    sudo dnf install -y git
fi

# NOTE: To avoid an error in "stanity-check" task, we will do modification
# on the file that is not tracked.
echo "Making copy of example inventory, then edit the file..."
cp tools/microshift/example-inventory.yaml tools/microshift/inventory.yaml
sed -i "s/ANSIBLE_USER/$ANSIBLE_USER/g" tools/microshift/inventory.yaml
sed -i "s/ANSIBLE_HOST/$ANSIBLE_HOST/g" tools/microshift/inventory.yaml

echo "Installing Ansible galaxy collection"
# NOTE: Workaround issues when ansible-galaxy silently fail to install a collection
for i in $(seq 10); do
    ansible-galaxy collection install --timeout=15 -vv \
        git+https://github.com/ansible-collections/community.general \
        git+https://github.com/ansible-collections/community.crypto \
        git+https://github.com/ansible-collections/ansible.posix
    INSTALLED=$(ansible-galaxy collection list)
    (echo $INSTALLED | grep -q community.general) && (echo $INSTALLED | grep -q community.crypto) && (echo $INSTALLED | grep -q ansible.posix) && break
    echo "Failed to install collection: $INSTALLED"
    sleep 1
done

echo "Clone ansible microshift role on localhost"
export ANSIBLE_LOG_PATH=ansible-clone-microshift-role.log
ansible-playbook \
    -i localhost \
    -e "hostname=localhost" \
    tools/microshift/ansible-microshift-role.yaml || fail "Failed to clone ansible-microshift-role!"

echo "Deploy and configure MicroShift on host $ANSIBLE_HOST..."
# NOTE: We include the setup-microshift/defaults/main.yaml variables, due
# on deploying ansible-microshift-role, it is ignoring default values from
# the role that triggered the role. In other words, default values from
# setup-microshfit role are ignored by ansible-microshift-role.
export ANSIBLE_LOG_PATH=ansible-do-microshift.log
ansible-playbook \
    -i tools/microshift/inventory.yaml \
    -e "@tools/microshift/roles/setup-microshift/defaults/main.yaml" \
    -e "@${PULL_SECRET_FILE_PATH}" \
    tools/microshift/do-microshift.yaml || fail "MicroShift deployment and configuration failed!"

echo "Deploying Microshift done! Pulling KUBECONFIG to ~/.kube/microshift-config..."
# take the config now to local env
mkdir -p ~/.kube
ansible controller \
    -i tools/microshift/inventory.yaml \
    -m "ansible.builtin.fetch" \
    -a '{"src": "~/.kube/microshift-config", "dest":"~/.kube/microshift-config", "flat":"true"}'
