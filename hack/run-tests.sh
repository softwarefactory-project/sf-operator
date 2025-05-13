#!/bin/sh -e
# Copyright © 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

# This is the main contributor interface to perform the sf-operator functional test.
# usage: ./hack/run-tests.sh

python3 -m pip --version >/dev/null 2>/dev/null || sudo dnf install -y python3-pip
python3 -c "import kubernetes" >/dev/null 2>/dev/null || python3 -m pip install kubernetes
ansible-galaxy collection install community.kubernetes ansible.posix
ansible-playbook -e "hostname=localhost" ./playbooks/run-tests.yaml
