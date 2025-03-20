#!/bin/sh -e
# Copyright © 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

if ! command -v ansible-playbook >/dev/null 2>&1; then
    if command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y ansible-core
    else
        echo "Unknown distribution, please update the 'hack/setup-ansible.sh'"
        exit 1
    fi
fi
