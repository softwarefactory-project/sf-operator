#!/bin/bash

set -ex

export PATH=$PATH:~/bin
alias kcadm=kcadm.sh

# TODO: For now download the jq binary. It might better to provision on the image
curl -sfL https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 -o ~/bin/jq && \
  chmod +x ~/bin/jq 

function get_user_id () {
  local username=$1
  kcadm get users --query "username=$username" --fields id -r SF | jq -r '.[0].id'
}

function set_user () {
  local username=$1
  local firstname=$2
  local lastname=$3
  local password=$4
  local is_admin=$5

  local uid=$(get_user_id $username)
  echo "Found $username uid: $uid"

  if [ "$uid" == "null" ]; then
    # Create SF realm's user
    kcadm create users --target-realm SF \
      --set "username=$username" \
      --set "email=$username@${FQDN}" \
      --set "firstName=$firstname" \
      --set "lastName=$lastname" \
      --set "enabled=true"
    local uid=$(get_user_id $username)
    echo "Created $username uid: $uid"
  fi

  # Set user password
  kcadm set-password --target-realm SF \
    --userid $uid \
    --new-password $password
  
  # Set user email (this refresh the email domain in case of SF domain change)
  kcadm update users/$uid --target-realm SF \
    --set "email=$username@${FQDN}"
  
  # Set user as administrator of the realm if admin option is "true"
  if [ "$is_admin" == "true" ]; then
    echo "Set $username as realm's administrator"
    kcadm add-roles --target-realm SF \
      --uusername `echo $username | tr '[:upper:]' '[:lower:]'` \
      --cclientid realm-management \
      --rolename manage-realm \
      --rolename manage-users
  fi
}

# Set credentials for the next commands
kcadm config credentials \
  --password ${KEYCLOAK_ADMIN_PASSWORD} \
  --realm master \
  --server https://keycloak:${KC_PORT} \
  --user ${KEYCLOAK_ADMIN} \
  --truststore /keycloak-data/keystore/truststore \
  --trustpass changeit

# Setup truststore config for next commands
kcadm config truststore /keycloak-data/keystore/truststore --trustpass changeit

# Create SF realm
kcadm get realms/SF > /dev/null || kcadm create realms --set realm=SF --set enabled=true

# Setup SF realm's admin user
set_user "admin" "Admin" "Software Factory" ${KEYCLOAK_SF_ADMIN_PASSWORD} "true"

# Setup SF realm's SF_SERVICE_USER user
set_user "SF_SERVICE_USER" "Service User" "Software Factory" ${KEYCLOAK_SF_SERVICE_PASSWORD} "true"
