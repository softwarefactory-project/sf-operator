#!/bin/sh

set -ex

exec /opt/keycloak/bin/kc.sh start-dev \
  --db mariadb \
	--db-url-database keycloak \
	--db-url-host mariadb \
	--db-username keycloak \
	--db-password ${DB_PASSWORD} \
  --health-enabled true \
  --metrics-enabled true \
  --hostname ${INGRESS_HOSTNAME} \
  --proxy passthrough \
  --https-key-store-file /keycloak-data/keystore/keystore \
  --https-key-store-password ${KC_KEYSTORE_PASSWORD} \
  --https-trust-store-file /keycloak-data/keystore/truststore \
  --https-trust-store-password changeit