#!/bin/bash

set -ex

# The /dev/./urandom is not a typo. https://stackoverflow.com/questions/58991966/what-java-security-egd-option-is-for
JAVA_OPTIONS="-Djava.security.egd=file:/dev/./urandom"

echo "Set local git config for gerrit admin"
cat << EOF > ~/.gitconfig
[user]
    name = SF initial configurator
    email = admin@${FQDN}
[gitreview]
    username = admin
[push]
    default = simple
EOF

echo "Setup .ssh/config to allow container exec of 'ssh gerrit'"
mkdir -p ~/.ssh
cat << EOF > ~/.ssh/config
Host gerrit
User admin
Hostname ${HOSTNAME}
Port 29418
IdentityFile ~/.ssh/gerrit_admin
EOF

echo "Copy Gerrit Admin SSH keys on filesystem"
echo "${GERRIT_ADMIN_SSH}" > ~/.ssh/gerrit_admin
chmod 0600 ~/.ssh/gerrit_admin

unset GERRIT_ADMIN_SSH

echo "Running Gerrit ..."
exec java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war daemon -d ~/