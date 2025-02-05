#!/bin/sh
# Copyright (C) 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

set -ex

export HOME=/var/lib/hound
mkdir -p ${HOME}/data
cd $HOME
if [ ! -z "${CONFIG_REPO_BASE_URL}" ]; then
    bash /sf-tooling/hound-search-config.sh
fi
if [ ! -f "/var/lib/hound/config.json" ]; then
    cat <<EOF> /var/lib/hound/config.json
{
 "dbpath": "/var/lib/hound/data",
 "max-concurrent-indexers": 2,
 "repos": {}
}
EOF
fi
exec /go/bin/houndd -conf /var/lib/hound/config.json
