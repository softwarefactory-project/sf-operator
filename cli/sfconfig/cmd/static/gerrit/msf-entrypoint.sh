#!/bin/sh

if [ "${HOME}" == "/" ]; then
    echo "HOME can not be / dir!"
    exit 1
fi

mkdir -p ~/.ssh
chmod 0700 ~/.ssh
echo "${SF_ADMIN_SSH}" > ~/.ssh/id_rsa
chmod 0400 ~/.ssh/id_rsa

cat << EOF > ~/.ssh/config
Host gerrit
User admin
Hostname ${GERRIT_SSHD_PORT_29418_TCP_ADDR}
Port ${GERRIT_SSHD_SERVICE_PORT_GERRIT_SSHD}
IdentityFile ~/.ssh/id_rsa
StrictHostKeyChecking no
EOF

sleep inf
