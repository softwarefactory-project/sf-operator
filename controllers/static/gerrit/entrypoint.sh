#!/bin/bash

set -ex

# The /dev/./urandom is not a typo. https://stackoverflow.com/questions/58991966/what-java-security-egd-option-is-for
JAVA_OPTIONS="-Djava.security.egd=file:/dev/./urandom"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.keyStore=/var/gerrit/etc/keystore"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.keyStorePassword=${GERRIT_KEYSTORE_PASSWORD}"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.trustStore=/var/gerrit/etc/truststore"
JAVA_OPTIONS="${JAVA_OPTIONS} -Djavax.net.ssl.trustStorePassword=changeit"

echo "Running Gerrit ..."
exec java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war daemon -d /var/gerrit