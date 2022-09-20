#!/bin/bash

# TODO:
# - Change database field type for SSH keys
# - Setup the SF theme - perhaps add the theme directory three in the container image
# - Get Postfix enabled in the sf-operator and ensure this pod get the POSTFIX_SERVICE_HOST env var
# - Perhaps do not manage the github provider config but instead document how to setup it in keycloak

set -ex

export PATH=$PATH:~/bin
alias kcadm=kcadm.sh

function get_user_id () {
  local username=$1
  kcadm get users --query "username=$username" --fields id -r SF | jq -r '.[0].id'
}

function get_client_id () {
  local clientid=$1
  kcadm get clients --query "clientId=$clientid" --fields id -r SF | jq -r '.[0].id'
}

function set_user () {
  local username=$1
  local firstname=$2
  local lastname=$3
  local password=$4
  local is_admin=$5

  local uid=$(get_user_id $username)
  echo "Found $username id: $uid"

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

function set_role () {
  local role_name=$1
  local role_desc=$2
  # If default is "true" all user in the REALM get assigned to that role
  local default=$3

  kcadm get roles/$role_name --fields id -r SF > /dev/null || \
    kcadm create roles --target-realm SF \
      --set "name=$role_name" \
      --set "description=$role_desc"

  if [ "$default" == "true" ]; then
    kcadm add-roles --target-realm SF \
      --rname default-roles-sf \
      --rolename $role_name
  fi
}

function assign_role_to_user () {
  local role_name=$1
  local username=$2

  kcadm add-roles --target-realm SF \
    --uusername `echo $username | tr '[:upper:]' '[:lower:]'` \
    --rolename $role_name
}

function create_oidc_client () {
  # clientid should by the service client name
  local clientid=$1

  local cid=$(get_client_id $clientid)
  echo "Found $clientid id: $cid"

  if [ "$cid" == "null" ]; then
    kcadm create clients --target-realm SF \
      --set clientId=$clientid \
      --set enabled=true \
      --set clientAuthenticatorType=client-secret
  fi
}

function set_oidc_client_origin () {
  local clientid=$1

  local cid=$(get_client_id $clientid)

  kcadm update clients/$cid --target-realm SF \
    --set "redirectUris=[\"https://${FQDN}/*\",\"https://$clientid.${FQDN}/*\"]" \
    --set "webOrigins=[\"https://${FQDN}\",\"https://$clientid.$FQDN\"]"
}

function set_oidc_client_secret () {
  local clientid=$1
  local secret=$2

  local cid=$(get_client_id $clientid)

  kcadm update clients/$cid --target-realm SF \
     --set secret=$secret
}

### Config kcadm client to communicate with the Keycloak API ###
################################################################

# Set credentials for the next commands
kcadm config credentials \
  --password ${KEYCLOAK_ADMIN_PASSWORD} \
  --realm master \
  --server https://keycloak \
  --user ${KEYCLOAK_ADMIN} \
  --truststore /keycloak-data/keystore/truststore \
  --trustpass changeit

# Setup truststore config for next commands
kcadm config truststore /keycloak-data/keystore/truststore --trustpass changeit

### Create the SF REALM ###
###########################

# Create SF realm
kcadm get realms/SF > /dev/null || kcadm create realms --set realm=SF --set enabled=true

### Setup default REALM users ###
#################################

# Setup SF realm's admin user (administrator)
set_user "admin" "Admin" "Software Factory" ${KEYCLOAK_SF_ADMIN_PASSWORD} "true"

# Setup SF realm's SF_SERVICE_USER user (administrator)
set_user "SF_SERVICE_USER" "Service User" "Software Factory" ${KEYCLOAK_SF_SERVICE_PASSWORD} "true"

# Setup SF realm's demo user (regular user)
set_user "demo" "Demo" "User" "demo" "false"

### Define some REALM roles ###
###############################

# Only set those roles when opensearch is deployed
env | grep OPENSEARCH && {
  set_role "kibana_viewer" "Default kibana viewer role" "true"
}

# Set an admin role
set_role "admin" "Admin access" "false"

# Assign the admin role to the admin user
assign_role_to_user "admin" "admin"

### Set password registration to in ###
#######################################

kcadm update realms/SF \
  --set "registrationAllowed=true" \
  --set "resetPasswordAllowed=true"

### Set default theme to SF custom ###
######################################
kcadm update realms/SF --set "loginTheme=sf" --set "accountTheme=sf"

### Set the SMTP config ###
###########################

kcadm update realms/SF \
  --set "smtpServer.host=${POSTFIX_SERVICE_HOST}" \
  --set "smtpServer.port=25" \
  --set "smtpServer.from=keycloak@${FQDN}" \
  --set "smtpServer.replyTo=admin@${FQDN}" \
  --set 'smtpServer.fromDisplayName="Software Factory IAM"'

### Setup MQTT Events listener ###
##################################

kcadm update events/config --target-realm SF \
  --set 'eventsListeners=["jboss-logging","mqtt"]' \
  --set eventsEnabled=true \
  --set enabledEventTypes=[]

### Create OIDC Client config ###
#################################

# Setup Gerrit client when a client secret is available in the env vars
if [ -n "${KEYCLOAK_GERRIT_CLIENT_SECRET}" ]; then
  create_oidc_client "gerrit"
  set_oidc_client_origin "gerrit"
  set_oidc_client_secret "gerrit" "${KEYCLOAK_GERRIT_CLIENT_SECRET}"
fi