#!/bin/sh
# Copyright (C) 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

set -ex

if [ ! -d /etc/pki/ca-trust/extracted/openssl ]; then
  cd /etc/pki/ca-trust/extracted
  mkdir -p openssl pem java edk2
  cd -
  update-ca-trust extract -o /etc/pki/ca-trust/extracted
fi

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
