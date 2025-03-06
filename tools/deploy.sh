#!/bin/sh -e
# Copyright © 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

# This is the main user interface to deploy the sf-operator on localhost.
# usage: ./tools/deploy.sh

# Host preparation
type -p kubectl > /dev/null || {
    echo "[+] Installing kubectl"
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod 555 kubectl
    sudo mv kubectl /bin
}
timeout 5s kubectl get pods 2>/dev/null >/dev/null > /dev/null || {
    echo "[+] Deploying minikube"
    ./tools/setup-minikube.sh localhost
}
go version > /dev/null || {
    echo "[+] Installing go"
    ansible-playbook -e "hostname=localhost" ./playbooks/install-golang.yaml
}

# Operator deployment
export OPENSHIFT_USER=false

echo "[+] Deploying sf-operator"
go run ./main.go dev --namespace sf create standalone-sf --cr ./playbooks/files/sf-minimal.yaml

# TODO: add a SF_OPERATOR_DEMO environment variable to skip gerrit deployment
grep -q " gerrit\." /etc/hosts > /dev/null || {
    echo "[+] Setting up gerrit"
    sudo chown $USER /etc/hosts
    go run main.go --config playbooks/files/sf-operator-cli.yaml dev create demo-env --repos-path deploy
    sudo chown root /etc/hosts
}
