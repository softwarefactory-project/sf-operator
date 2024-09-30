#!/bin/sh

set -ex

# Update the CA Trust chain
update-ca-trust extract -o /etc/pki/ca-trust/extracted

# This create some directory expected by nodepool-builder
mkdir -p ~/dib ~/nodepool/builds

# Generate the Nodepool configuration
/usr/local/bin/generate-config.sh
