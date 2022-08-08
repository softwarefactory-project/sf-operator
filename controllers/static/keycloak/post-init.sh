#!/bin/bash

set -ex

env

export KC_ADM="/opt/jboss/keycloak/bin/kcadm.sh"

# Set credentials for folowing commands
${KC_ADM} config credentials --password ${KC_ADMIN_PASS} --realm master --server http://keycloak:${KC_PORT}/auth --user admin

# # Create SF realm
${KC_ADM} create realms --set realm=SF --set enabled=true