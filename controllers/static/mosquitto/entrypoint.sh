#!/bin/bash

set -ex

env

cat << EOF > /etc/mosquitto/acl.conf
topic read #
user SF_SERVICE_USER
topic readwrite #
#user SF_3RD_PARTY
#topic readwrite 3rdparty/#
EOF

cat << EOF > /etc/mosquitto/mosquitto.conf
log_dest stdout
listener 1883
listener 1884
#Check if it is really necessary to have a websocket port open
#protocol websockets
http_dir /home/mosquitto
acl_file /etc/mosquitto/acl.conf
password_file /etc/mosquitto/passwords
# In version 1.6.x and earlier, this option defaulted to true
allow_anonymous true
EOF

touch /etc/mosquitto/passwords

mosquitto_passwd -b /etc/mosquitto/passwords SF_SERVICE_USER sfserviceuserpass

mosquitto -v -c /etc/mosquitto/mosquitto.conf
