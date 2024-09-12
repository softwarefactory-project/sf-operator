#!/bin/sh

set -ex

# Update the CA Trust chain
update-ca-trust extract -o /etc/pki/ca-trust/extracted

# This is needed when we mount the local zuul source from the host
# to bypass the git ownership verification
# https://git-scm.com/docs/git-config#Documentation/git-config.txt-safedirectory
git config --global --add safe.directory $HOME/config

# Generate the Zuul tenant configuration
/usr/local/bin/generate-zuul-tenant-yaml.sh