#!/bin/sh

set -ex

# Update the CA Trust chain
mkdir -p /etc/pki/ca-trust/extracted/{pem,java,edk2,openssl}
update-ca-trust

# This is needed when we mount the local zuul source from the host
# to bypass the git ownership verification
# https://git-scm.com/docs/git-config#Documentation/git-config.txt-safedirectory
git config --global --add safe.directory $HOME/config

# Generate the Zuul tenant configuration
/usr/local/bin/generate-zuul-tenant-yaml.sh