#!/bin/bash
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

# Same as above, but client-scoped. Notice client ID is the first parameter
function set_client_scoped_role () {
  local clientid=$1
  local role_name=$2
  local role_desc=$3
  local default=$4

  local cid=$(get_client_id $clientid)
  kcadm get-roles --rolename $role_name --cclientid $1 --fields id -r SF > /dev/null || \
    kcadm create clients/${cid}/roles --target-realm SF \
      --set "name=$role_name" \
      --set "description=$role_desc"

  if [ "$default" == "true" ]; then
    kcadm add-roles --target-realm SF --cclientid $clientid \
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

function create_oidc_client_with_secret () {
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

function create_oidc_public_client () {
  # clientid should by the service client name
  local clientid=$1

  local cid=$(get_client_id $clientid)
  echo "Found $clientid id: $cid"

  if [ "$cid" == "null" ]; then
    kcadm create clients --target-realm SF \
      --set clientId=$clientid \
      --set enabled=true \
      --set publicClient=true \
      --set implicitFlowEnabled=true
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

function get_client_scope_id () {
  local scope_name=$1

  kcadm get client-scopes --target-realm SF \
    --fields name,id | jq -r -c ".[] | select (.name == \"${scope_name}\").id"
}

function create_client_scope () {
  local clientid=$1
  local scope_name=$2

  local sid=$(get_client_scope_id ${scope_name})

  if [ -z "$sid" ]; then
    local sid=$(kcadm create client-scopes --target-realm SF \
      --set "name=${scope_name}" \
      --set "protocol=openid-connect" \
      --set "attributes.\"include.in.token.scope\"=true" \
      --set "attributes.\"display.on.consent.screen\"=false" \
      -o --fields id | jq -r '.id')
  fi

  # By default Keycloak does not set the "aud" claim to the client ID.
  # This is however sometimes the expected behavior with some libraries
  # when validating JWTs (pyJWT does it for example). To avoid problems
  # we add a mapper to the scope that will add the client ID in the "aud"
  # claim of the ID token.
  local mid=$(kcadm get client-scopes/${sid} --target-realm SF \
    | jq -r '.protocolMappers | .[]? | select (.name == "audience_mapper") | .id')
  if [ -z "$mid" ]; then
    kcadm create client-scopes/${sid}/protocol-mappers/models \
      --target-realm SF \
      --set name=audience_mapper \
      --set protocol=openid-connect \
      --set protocolMapper=oidc-audience-mapper \
      --set 'config."id.token.claim"=true' \
      --set 'config."access.token.claim"=true' \
      --set "config.\"included.client.audience\"=${clientid}"
  fi
}

function configure_oidc_client_extra_scope () {
  local clientid=$1
  local extra_scope=$2
  local scopes="\"web-origins\", \"profile\", \"roles\", \"email\", \"${extra_scope}\""

  local cid=$(get_client_id $clientid)

  kcadm update clients/$cid --target-realm SF \
    --set "defaultClientScopes=[$scopes]"

  local sid=$(get_client_scope_id ${extra_scope})

  kcadm update clients/${cid}/default-client-scopes/${sid} --target-realm SF
}

function add_mapper () {
  # Use this function to add a custom claim to the userinfo, access and id
  # tokens based on an existing protocol mapper. Existing protocol mappers include:
  # - for realm roles: oidc-usermodel-realm-role-mapper
  # - for realm groups: oidc-group-membership-mapper

  local clientid=$1
  local claim_name=$2
  local protocol_mapper=$3

  local cid=$(get_client_id $clientid)

  local mid=$(kcadm get clients/${cid}/protocol-mappers/models --target-realm SF \
    | jq -r ".[]? | select (.name == \"${claim_name}\") | .id")
  if [ -z "$mid" ]; then
    kcadm create clients/${cid}/protocol-mappers/models --target-realm SF \
      --set name=${claim_name} \
      --set protocol=openid-connect \
      --set protocolMapper=${protocol_mapper} \
      --set consentRequired=false \
      --set 'config."multivalued"=true' \
      --set 'config."userinfo.token.claim"=true' \
      --set 'config."id.token.claim"=true' \
      --set 'config."access.token.claim"=true' \
      --set "config.\"claim.name\"=${claim_name}" \
      --set 'config."jsonType.label"=String'
  fi
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

# Set an admin role
set_role "admin" "Admin access" "false"

# Assign the admin role to the admin user
assign_role_to_user "admin" "admin"

### Set password registration to on ###
#######################################

kcadm update realms/SF \
  --set "registrationAllowed=true" \
  --set "resetPasswordAllowed=true"

### Set default theme to SF custom ###
######################################
kcadm update realms/SF --set "loginTheme=sf" --set "accountTheme=sf"

### Disable RSA OAEP key signing algorithm globally ###
#######################################################

# As of Zuul 6 the pyJWT dependency does not support JWTs
# signed with the RSA-OAEP algorithm, see https://github.com/jpadilla/pyjwt/issues/722
kid=$(kcadm.sh get components --target-realm SF | jq -r ".[] | select (.name == \"rsa-enc-generated\").id")
kcadm.sh update components/${kid} \
  --target-realm SF \
  --set "config.active=[\"false\"]" \
  --set "config.enabled=[\"false\"]"

### Setup MQTT Events listener ###
##################################

kcadm update events/config --target-realm SF \
  --set 'eventsListeners=["jboss-logging","mqtt"]' \
  --set eventsEnabled=true \
  --set enabledEventTypes=[]

### Configure services for OIDC authentication ###
##################################################

# Gerrit support
if [ -n "${KEYCLOAK_GERRIT_CLIENT_SECRET}" ]; then
  create_oidc_client_with_secret "gerrit"
  set_oidc_client_origin "gerrit"
  set_oidc_client_secret "gerrit" "${KEYCLOAK_GERRIT_CLIENT_SECRET}"
fi

# Zuul support
if [ "${ZUUL_ENABLED}" == "true" ]; then
  create_oidc_public_client "zuul"
  set_oidc_client_origin "zuul"
  create_client_scope "zuul" "zuul_keycloak_scope"
  configure_oidc_client_extra_scope "zuul" "zuul_keycloak_scope"
  add_mapper "zuul" "roles" "oidc-usermodel-realm-role-mapper"
  # Create "admin on every tenant" role
  set_client_scoped_role "zuul" "zuul_admin" "This role grants privileged actions such as dequeues and autoholds on every Zuul tenant" "false"
fi
