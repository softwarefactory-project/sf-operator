#!/bin/sh -e
# Copyright © 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

# This is the main contributor interface to perform the sf-operator functional test.
# usage: ./hack/run-tests.sh

export OPENSHIFT_USER=false
ansible-playbook -e "hostname=localhost" ./playbooks/run-tests.yaml
