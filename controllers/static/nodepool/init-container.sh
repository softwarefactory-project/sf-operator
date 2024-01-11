#!/bin/sh

set -ex

# Update the CA Trust chain
mkdir -p /etc/pki/ca-trust/extracted/{pem,java,edk2,openssl}
update-ca-trust

# This create some directory expected by nodepool-builder
mkdir -p ~/dib ~/nodepool/builds

# Generate the Nodepool configuration
/usr/local/bin/generate-config.sh