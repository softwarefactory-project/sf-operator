#!/bin/sh

set -ex

# Update the CA Trust chain
mkdir -p /etc/pki/ca-trust/extracted/{pem,java,edk2,openssl}
update-ca-trust

# Generate the Zuul tenant configuration
/usr/local/bin/generate-zuul-tenant-yaml.sh