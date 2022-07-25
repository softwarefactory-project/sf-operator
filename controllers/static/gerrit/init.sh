#!/bin/bash

set -ex

# The /dev/./urandom is not a typo. https://stackoverflow.com/questions/58991966/what-java-security-egd-option-is-for
JAVA_OPTIONS="-Djava.security.egd=file:/dev/./urandom"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.keyStore=/var/gerrit/etc/keystore"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.keyStorePassword=${GERRIT_KEYSTORE_PASSWORD}"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.trustStore=/var/gerrit/etc/truststore"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.trustStorePassword=changeit"

echo "Initializing the truststore ..."
rm -f /var/gerrit/etc/truststore
rm -f /var/gerrit/etc/keystore
cat /var/gerrit/cert/ca.crt /var/gerrit/cert/tls.crt > /tmp/cert-chain.txt
openssl pkcs12 -export \
  -inkey /var/gerrit/cert/tls.key \
  -in /tmp/cert-chain.txt -out /tmp/tls.pkcs12 \
  -passout pass:${GERRIT_KEYSTORE_PASSWORD}
keytool -importkeystore -srckeystore /tmp/tls.pkcs12 \
  -srcstoretype PKCS12 -destkeystore /var/gerrit/etc/keystore \
  -srcstorepass ${GERRIT_KEYSTORE_PASSWORD} -deststorepass ${GERRIT_KEYSTORE_PASSWORD}
keytool -importcert -alias ${FQDN}-root-ca \
  -file /var/gerrit/cert/ca.crt \
  -keystore /var/gerrit/etc/truststore -storepass changeit -noprompt

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
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.image/*.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.text/xml.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.text/plain.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.text/css.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.text/x-yaml.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.text/x-rst.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.text/x-puppet.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.text/x-ini.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.text/x-properties.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.text/x-markdown.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all mimetype.application/xml.safe "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all user.email "gerrit@${FQDN}"
git config -f /var/gerrit/etc/gerrit.config --replace-all suggest.from "2"
git config -f /var/gerrit/etc/gerrit.config --replace-all sendmail.enable "true"
git config -f /var/gerrit/etc/gerrit.config --replace-all sendmail.from "MIXED"
git config -f /var/gerrit/etc/gerrit.config --replace-all sendmail.smtpServer "postfix"
git config -f /var/gerrit/etc/gerrit.config --replace-all commentlink.testresult.match "<li>([^ ]+) <a href=\"[^\"]+\" target=\"_blank\" rel=\"nofollow\">([^<]+)</a> : ([^ ]+)([^<]*)</li>"
git config -f /var/gerrit/etc/gerrit.config --replace-all commentlink.testresult.html "<li class=\"comment_test\"><span class=\"comment_test_name\"><a href=\"\$2\" rel=\"nofollow\">\$1</a></span> <span class=\"comment_test_result\"><span class=\"result_\$3\">\$3</span>\$4</span></li>"
git config -f /var/gerrit/etc/gerrit.config --replace-all commentlink.changeid.match "(I[0-9a-f]{8,40})"
git config -f /var/gerrit/etc/gerrit.config --replace-all commentlink.changeid.html "#/q/\$1"
