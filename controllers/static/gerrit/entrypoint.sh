#!/bin/bash

set -ex

# The /dev/./urandom is not a typo. https://stackoverflow.com/questions/58991966/what-java-security-egd-option-is-for
JAVA_OPTIONS="-Djava.security.egd=file:/dev/./urandom"

echo "Initializing Gerrit site ..."
java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war init -d /var/gerrit --batch --no-auto-start --skip-plugins
java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war reindex -d /var/gerrit

echo "Installing plugins ..."
unzip -jo /var/gerrit/bin/gerrit.war WEB-INF/plugins/* -d /var/gerrit/plugins
for plugin in /var/gerrit-plugins/*; do
		cp -uv $plugin /var/gerrit/plugins/
done

echo "Creating admin account if needed"
cat << EOF > /var/gerrit/.gitconfig
[user]
    name = SF initial configurator
    email = admin@${FQDN}
[gitreview]
    username = admin
[push]
    default = simple
EOF

echo "Ensure admin user"
pynotedb create-admin-user --email "admin@${FQDN}" --pubkey "${GERRIT_ADMIN_SSH_PUB}" \
  --all-users "/var/gerrit/git/All-Users.git" --scheme gerrit

echo "Copy Gerrit Admin SSH keys on filesystem"
echo "${GERRIT_ADMIN_SSH_PUB}" > /var/gerrit/.ssh/gerrit_admin.pub
chmod 0644 /var/gerrit/.ssh/gerrit_admin.pub
echo "${GERRIT_ADMIN_SSH}" > /var/gerrit/.ssh/gerrit_admin
chmod 0600 /var/gerrit/.ssh/gerrit_admin

echo "Setup .ssh/config to allow container exec of 'ssh gerrit'"
cat << EOF > /var/gerrit/.ssh/config
Host gerrit
User admin
Hostname ${HOSTNAME}
Port 29418
IdentityFile /var/gerrit/.ssh/gerrit_admin
EOF

echo "Setting Gerrit config file ..."
git config -f /var/gerrit/etc/gerrit.config --replace-all auth.type "DEVELOPMENT_BECOME_ANY_ACCOUNT"
# git config -f /var/gerrit/etc/gerrit.config --replace-all auth.type "HTTP"
git config -f /var/gerrit/etc/gerrit.config --replace-all sshd.listenaddress "*:29418"
git config -f /var/gerrit/etc/gerrit.config --replace-all sshd.idleTimeout "2d"
git config -f /var/gerrit/etc/gerrit.config --replace-all sshd.maxConnectionsPerUser "${SSHD_MAX_CONNECTIONS_PER_USER:-10}"

echo "Running Gerrit ..."
exec java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war daemon -d /var/gerrit