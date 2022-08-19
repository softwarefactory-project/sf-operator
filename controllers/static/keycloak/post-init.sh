#!/bin/bash

set -ex

export KC_ADM="/opt/keycloak/bin/kcadm.sh"

# Set credentials for folowing commands
${KC_ADM} config credentials \
  --password ${KEYCLOAK_ADMIN_PASSWORD} \
  --realm master \
  --server https://keycloak:${KC_PORT} \
  --user ${KEYCLOAK_ADMIN} \
  --truststore /keycloak-data/keystore/truststore \
  --trustpass changeit

# Setup truststore config for further commands
${KC_ADM} config truststore /keycloak-data/keystore/truststore --trustpass changeit

# Create SF realm
${KC_ADM} get realms/SF > /dev/null || ${KC_ADM} create realms --set realm=SF --set enabled=true