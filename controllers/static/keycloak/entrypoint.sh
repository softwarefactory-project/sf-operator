#!/bin/sh

set -ex

# Install a script to ease using kcadm
cat << EOF > ~/bin/set-kcadm.sh
#!/bin/bash

# Set credentials for the next commands
/opt/keycloak/bin/kcadm.sh config credentials \
  --password \${KEYCLOAK_ADMIN_PASSWORD} \
  --realm master \
  --server https://keycloak \
  --user ${KEYCLOAK_ADMIN} \
  --truststore /keycloak-data/keystore/truststore \
  --trustpass changeit

# Setup truststore config for next commands
/opt/keycloak/bin/kcadm.sh config truststore /keycloak-data/keystore/truststore --trustpass changeit

echo "run: alias kcadm=/opt/keycloak/bin/kcadm.sh"
EOF
chmod +x ~/bin/set-kcadm.sh

exec /opt/keycloak/bin/kc.sh start \
  --log-level info \
  --db mariadb \
	--db-url-database keycloak \
	--db-url-host mariadb \
	--db-username keycloak \
	--db-password ${DB_PASSWORD} \
  --health-enabled true \
  --metrics-enabled true \
  --hostname ${INGRESS_HOSTNAME} \
  --proxy edge \
  --https-key-store-file /keycloak-data/keystore/keystore \
  --https-key-store-password ${KC_KEYSTORE_PASSWORD} \
  --https-trust-store-file /keycloak-data/keystore/truststore \
  --https-trust-store-password changeit
