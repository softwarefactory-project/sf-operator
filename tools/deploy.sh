#!/bin/sh
# Copyright © 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

# This is the main user interface to deploy the sf-operator on localhost.
# usage: ./tools/deploy.sh

./tools/setup-minikube.sh localhost
go --version || {
    ansible-playbook -e "hostname=localhost" ./playbooks/install-golang.yaml
}

OPENSHIFT_USER=false go run ./main.go dev create standalone-sf --cr ./playbooks/files/sf-minimal.yaml
