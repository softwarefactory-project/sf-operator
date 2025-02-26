#!/bin/bash -e
# Copyright © 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

# This is the main user interface to remove sf-operator from localhost.
# usage: ./tools/destroy.sh

read -p "Are you sure you want to destroy your local deployment (y/N)? " -n 1 -r
if [[ $REPLY =~ ^[Yy]$ ]]
then
  sudo sed '/ gerrit\./d' -i /etc/hosts
  rm -Rf deploy
  minikube delete
fi
