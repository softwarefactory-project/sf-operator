#!/bin/bash

set -ex

GERRIT_SITE="/gerrit"
GERRIT_CERT="/gerrit-cert"

# The /dev/./urandom is not a typo. https://stackoverflow.com/questions/58991966/what-java-security-egd-option-is-for
JAVA_OPTIONS="-Djava.security.egd=file:/dev/./urandom"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.keyStore=${GERRIT_SITE}/etc/keystore"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.keyStorePassword=${GERRIT_KEYSTORE_PASSWORD}"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.trustStore=${GERRIT_SITE}/etc/truststore"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.trustStorePassword=changeit"

echo "Initializing the truststore ..."
rm -f ${GERRIT_SITE}/etc/truststore
rm -f ${GERRIT_SITE}/etc/keystore
mkdir -p ${GERRIT_SITE}/etc
cat ${GERRIT_CERT}/ca.crt ${GERRIT_CERT}/tls.crt > /tmp/cert-chain.txt
openssl pkcs12 -export \
  -inkey ${GERRIT_CERT}/tls.key \
  -in /tmp/cert-chain.txt -out /tmp/tls.pkcs12 \
  -passout pass:${GERRIT_KEYSTORE_PASSWORD}
keytool -importkeystore -srckeystore /tmp/tls.pkcs12 \
  -srcstoretype PKCS12 -destkeystore ${GERRIT_SITE}/etc/keystore \
  -srcstorepass ${GERRIT_KEYSTORE_PASSWORD} -deststorepass ${GERRIT_KEYSTORE_PASSWORD}
keytool -importcert -alias ${FQDN}-root-ca \
  -file ${GERRIT_CERT}/ca.crt \
  -keystore ${GERRIT_SITE}/etc/truststore -storepass changeit -noprompt

echo "Initializing Gerrit site ..."
java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war init -d ${GERRIT_SITE} --batch --no-auto-start --skip-plugins
java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war reindex -d ${GERRIT_SITE}

echo "Installing plugins ..."
cp -u /var/gerrit-plugins/* ${GERRIT_SITE}/plugins

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

echo "Ensure admin user"
pynotedb create-admin-user --email "admin@${FQDN}" --pubkey "${GERRIT_ADMIN_SSH_PUB}" \
  --all-users "${GERRIT_SITE}/git/All-Users.git" --scheme gerrit

echo "Copy Gerrit Admin SSH keys on filesystem"
mkdir -p ${HOME}/.ssh
echo "${GERRIT_ADMIN_SSH_PUB}" > ${HOME}/.ssh/gerrit_admin.pub
chmod 0644 ${HOME}/.ssh/gerrit_admin.pub
echo "${GERRIT_ADMIN_SSH}" > ${HOME}/.ssh/gerrit_admin
chmod 0600 ${HOME}/.ssh/gerrit_admin

echo "Setup .ssh/config to allow container exec of 'ssh gerrit'"
cat << EOF > ${HOME}/.ssh/config
Host gerrit
User admin
Hostname ${HOSTNAME}
Port 29418
IdentityFile ${HOME}/.ssh/gerrit_admin
EOF

echo "Setting Gerrit config file ..."
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all auth.type "DEVELOPMENT_BECOME_ANY_ACCOUNT"
# git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all auth.type "HTTP"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all sshd.listenaddress "*:29418"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all sshd.idleTimeout "2d"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all sshd.maxConnectionsPerUser "${SSHD_MAX_CONNECTIONS_PER_USER:-10}"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.image/*.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.text/xml.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.text/plain.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.text/css.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.text/x-yaml.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.text/x-rst.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.text/x-puppet.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.text/x-ini.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.text/x-properties.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.text/x-markdown.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all mimetype.application/xml.safe "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all user.email "gerrit@${FQDN}"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all suggest.from "2"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all sendmail.enable "true"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all sendmail.from "MIXED"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all sendmail.smtpServer "postfix"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all commentlink.testresult.match "<li>([^ ]+) <a href=\"[^\"]+\" target=\"_blank\" rel=\"nofollow\">([^<]+)</a> : ([^ ]+)([^<]*)</li>"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all commentlink.testresult.html "<li class=\"comment_test\"><span class=\"comment_test_name\"><a href=\"\$2\" rel=\"nofollow\">\$1</a></span> <span class=\"comment_test_result\"><span class=\"result_\$3\">\$3</span>\$4</span></li>"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all commentlink.changeid.match "(I[0-9a-f]{8,40})"
git config -f ${GERRIT_SITE}/etc/gerrit.config --replace-all commentlink.changeid.html "#/q/\$1"
