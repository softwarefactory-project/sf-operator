#!/bin/sh -e
# Copyright © 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

VERSION=$1

if [ -z "$VERSION" ]; then
    echo $0 version;
    exit 1
fi

if [ "$VERSION" == "v0.0.62" ]; then
    # The zuul executor loose ZK connection right before scale down, which prevents it to stop before the graceful period.
    # The following code ensure the process stops.
    ( sleep 60; sudo kill -9 $(ps auxw | grep python3.*zuul-executor | awk '{ print $2 }');) &
fi

echo "[+] Setting up sf-operator $VERSION in deploy/upgrade-operator"
rm -Rf deploy/upgrade-operator && mkdir -p deploy/upgrade-operator
git clone . deploy/upgrade-operator/
cd deploy/upgrade-operator
git checkout $VERSION
exec ./hack/deploy.sh
