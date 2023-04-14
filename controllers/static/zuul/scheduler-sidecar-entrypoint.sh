#!/bin/sh

if [ "${HOME}" == "/" ]; then
    echo "HOME can not be / dir!"
    exit 1
fi

mkdir -p "${HOME}/.ssh"
chmod 0700 "${HOME}/.ssh"
echo "${SF_ADMIN_SSH}" > "${HOME}/.ssh/id_rsa"
chmod 0400 "${HOME}/.ssh/id_rsa"

touch /tmp/healthy

sleep inf
