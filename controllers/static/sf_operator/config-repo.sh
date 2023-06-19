#!/bin/bash

# This script is the entry point to populate a SF config repository
# according the config-locations spec. For the moment that's a shell script
# but we can rewrite it into python to make it a bit cleaner.

set -ex

env

if [ "${HOME}" == "/" ]; then
    echo "HOME can not be / dir!"
    exit 1
fi

cd ${HOME}

cat /etc/hosts

mkdir .ssh
chmod 0700 .ssh
echo "${SF_ADMIN_SSH}" > .ssh/id_rsa
chmod 0400 .ssh/id_rsa

cat << EOF > .gitconfig
[user]
    name = SF initial configurator
    email = admin@${FQDN}
[push]
    default = simple
EOF

export GIT_SSH_COMMAND="ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

git clone ssh://${CONFIG_REPO_USER}@${CONFIG_REPO_URL}

cd config

# Initialize zuul directory
mkdir -p zuul
touch zuul/main.yaml

# Initialize system resource
mkdir -p system
cat /sf-provided-cr/sf.yaml > system/sf.yaml

# Initialize zuul.d
mkdir -p zuul.d
touch zuul.d/jobs.yaml

git add -A
git commit -m"Populate config repository" && git push origin master || true
