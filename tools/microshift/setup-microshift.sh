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

if ! command -v ansible-playbook >/dev/null 2>&1; then
    # Detect distribution
    DIST=$(awk -F= '/^ID=/ {print $2}' /etc/os-release | tr -d '"' | tr '[:upper:]' '[:lower:]')

    case "$DIST" in
        "rhel")
            echo "Installing ansible-core on RHEL..."
            sudo dnf install -y ansible-core || fail "Failed to install ansible-core"
            ;;
        *)
            fail "Unsupported distribution: $DIST"
            ;;
    esac
fi

# NOTE: To avoid an error in "stanity-check" task, we will do modification
# on the file that is not tracked.
cp tools/microshift/example-inventory.yaml tools/microshift/inventory.yaml
sed -i "s/ANSIBLE_USER/$ANSIBLE_USER/g" tools/microshift/inventory.yaml
sed -i "s/ANSIBLE_HOST/$ANSIBLE_HOST/g" tools/microshift/inventory.yaml

ansible-galaxy collection install community.general community.crypto ansible.posix

echo "Clone ansible microshift role on localhost"
ansible-playbook \
    -i localhost \
    -e "hostname=localhost" \
    tools/microshift/ansible-microshift-role.yaml || fail "Failed to clone ansible-microshift-role!"

echo "Deploy and configure MicroShift on host $ANSIBLE_HOST..."
# NOTE: We include the setup-microshift/defaults/main.yaml variables, due
# on deploying ansible-microshift-role, it is ignoring default values from
# the role that triggered the role. In other words, default values from
# setup-microshfit role are ignored by ansible-microshift-role.
ansible-playbook \
    -i tools/microshift/inventory.yaml \
    -e "@tools/microshift/roles/setup-microshift/defaults/main.yaml" \
    -e "@${PULL_SECRET_FILE_PATH}" \
    tools/microshift/do-microshift.yaml || fail "MicroShift deployment and configuration failed!"

# take the config now to local env
mkdir -p ~/.kube
ansible controller \
    -i tools/microshift/inventory.yaml \
    -m "ansible.builtin.fetch" \
    -a '{"src": "~/.kube/microshift-config", "dest":"~/.kube/microshift-config", "flat":"true"}'
