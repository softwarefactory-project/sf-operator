#!/bin/sh

mkdir $HOME/.ssh
chmod 0700 $HOME/.ssh
echo "${SF_ADMIN_SSH}" > $HOME/.ssh/id_rsa
chmod 0400 $HOME/.ssh/id_rsa

touch /tmp/healthy

sleep inf