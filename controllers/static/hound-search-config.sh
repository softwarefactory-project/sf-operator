#!/bin/sh
# Copyright (C) 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

set -ex

export HOME=/var/lib/hound
bash /sf-tooling/fetch-config-repo.sh $1
exec python3 /sf-tooling/hound-search-render.py
