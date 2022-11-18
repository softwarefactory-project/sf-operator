#!/bin/bash

set -ex

GERRIT_SITE="/gerrit"

# The /dev/./urandom is not a typo. https://stackoverflow.com/questions/58991966/what-java-security-egd-option-is-for
JAVA_OPTIONS="-Djava.security.egd=file:/dev/./urandom"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.keyStore=${GERRIT_SITE}/etc/keystore"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.keyStorePassword=${GERRIT_KEYSTORE_PASSWORD}"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.trustStore=${GERRIT_SITE}/etc/truststore"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.trustStorePassword=changeit"

echo "Creating admin account if needed"
cat << EOF > ${HOME}/.gitconfig
[user]
    name = SF initial configurator
    email = admin@${FQDN}
[gitreview]
    username = admin
[push]
    default = simple
EOF

echo "Setup .ssh/config to allow container exec of 'ssh gerrit'"
mkdir -p ${HOME}/.ssh
cat << EOF > ${HOME}/.ssh/config
Host gerrit
User admin
Hostname ${HOSTNAME}
Port 29418
IdentityFile ${HOME}/.ssh/gerrit_admin

Host ${FQDN}
User admin
Hostname ${HOSTNAME}
Port 29418
IdentityFile ${HOME}/.ssh/gerrit_admin
EOF

echo "Copy Gerrit Admin SSH keys on filesystem"
echo "${GERRIT_ADMIN_SSH_PUB}" > ${HOME}/.ssh/gerrit_admin.pub
chmod 0644 ${HOME}/.ssh/gerrit_admin.pub
echo "${GERRIT_ADMIN_SSH}" > ${HOME}/.ssh/gerrit_admin
chmod 0600 ${HOME}/.ssh/gerrit_admin

unset GERRIT_ADMIN_SSH
unset GERRIT_ADMIN_SSH_PUB

echo "Running Gerrit ..."
exec java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war daemon -d ${GERRIT_SITE}