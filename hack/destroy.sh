#!/bin/bash -e
# Copyright © 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

# This is the main user interface to remove sf-operator from localhost.
# usage: ./hack/destroy.sh

read -p "Are you sure you want to destroy your local deployment (y/N)? " -n 1 -r
if [[ $REPLY =~ ^[Yy]$ ]]
then
  sudo sed '/ gerrit\./d' -i /etc/hosts
  rm -Rf deploy
  minikube delete || true
  sudo rm -Rf /dev/shm/etcd
  sudo systemctl stop minikube || true
  sudo systemctl disable minikube || true
  sudo rm -rf /etc/systemd/system/minikube.service
  echo "If hack/deploy.sh doesn't work, delete further the cache by running:"
  echo "sudo rm -Rf ~/.minikube /var/lib/containers/storage/volumes/minikube"
fi
