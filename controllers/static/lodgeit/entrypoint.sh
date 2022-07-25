#!/bin/bash

set -ex

env

cat << EOF > /etc/lodgeit/lodgeit.conf
[root]
dburi=mysql+pymysql://lodgeit:${LODGEIT_MYSQL_PASSWORD}@mariadb:3306/lodgeit
secret_key=${LODGEIT_SESSION_KEY}
EOF

lodgeit runserver -h ${HOSTNAME}
