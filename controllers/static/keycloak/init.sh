#!/bin/bash

set -ex

BASEDIR="/keycloak-data/keystore"

KEYCLOAK_CERT="/keycloak-cert"

KEYSTORE="${BASEDIR}/keystore"
TRUSTSTORE="${BASEDIR}/truststore"

# We re-init the store from the cert/key from certmanager
rm -f ${KEYSTORE}
rm -f ${TRUSTSTORE}

mkdir -p ${BASEDIR}

echo "Initializing the truststore ..."
cat ${KEYCLOAK_CERT}/ca.crt ${KEYCLOAK_CERT}/tls.crt > /tmp/cert-chain.txt
openssl pkcs12 -export \
  -inkey ${KEYCLOAK_CERT}/tls.key \
  -in /tmp/cert-chain.txt -out /tmp/tls.pkcs12 \
  -passout pass:${KC_KEYSTORE_PASSWORD}
keytool -importkeystore -srckeystore /tmp/tls.pkcs12 \
  -srcstoretype PKCS12 -destkeystore ${KEYSTORE} \
  -srcstorepass ${KC_KEYSTORE_PASSWORD} -deststorepass ${KC_KEYSTORE_PASSWORD}
keytool -importcert -alias ${FQDN}-root-ca \
  -file ${KEYCLOAK_CERT}/ca.crt \
  -keystore ${TRUSTSTORE} -storepass changeit -noprompt