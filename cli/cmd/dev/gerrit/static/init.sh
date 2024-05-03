#!/bin/bash

set -ex

export HOME=/gerrit
# The /dev/./urandom is not a typo. https://stackoverflow.com/questions/58991966/what-java-security-egd-option-is-for
JAVA_OPTIONS="-Djava.security.egd=file:/dev/./urandom -Xms${JVM_XMS} -Xmx${JVM_XMX}"

echo "Initializing Gerrit site ..."
java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war init -d ~/ --batch --no-auto-start --skip-plugins

echo "Installing plugins ..."
cp -u /var/gerrit-plugins/* ~/plugins

cat << EOF > ~/.gitconfig
[user]
    name = SF initial configurator
    email = admin@${FQDN}
[gitreview]
    username = admin
[push]
    default = simple
EOF

echo "Ensure admin user"
# This command is noop if admin user already exists
pynotedb create-admin-user --email "admin@${FQDN}" --pubkey "${GERRIT_ADMIN_SSH_PUB}" \
  --all-users ~/git/All-Users.git --scheme gerrit

echo "Setting Gerrit config file ..."
git config -f ~/etc/gerrit.config --replace-all gerrit.canonicalWebUrl "https://gerrit.${FQDN}"
git config -f ~/etc/gerrit.config --replace-all auth.type "DEVELOPMENT_BECOME_ANY_ACCOUNT"
git config -f ~/etc/gerrit.config --replace-all sshd.listenaddress "*:29418"
git config -f ~/etc/gerrit.config --unset-all httpd.listenUrl
git config -f ~/etc/gerrit.config --add httpd.listenUrl "proxy-https://*:8080/"
git config -f ~/etc/gerrit.config --replace-all user.email "gerrit@${FQDN}"
git config -f ~/etc/gerrit.config --replace-all sendemail.enable "false"

echo "Install the ready.sh script"
cat << EOF > ~/ready.sh
echo "Waiting for httpd"
curl --fail http://localhost:8080/config/server/version

echo "Waiting for sshd"
python3 -c 'import socket; socket.socket(socket.AF_INET, socket.SOCK_STREAM).connect(("localhost", 29418))'
EOF
chmod +x ~/ready.sh