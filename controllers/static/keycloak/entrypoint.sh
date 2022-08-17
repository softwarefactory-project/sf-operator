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
  --proxy passthrough