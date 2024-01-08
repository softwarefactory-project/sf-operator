#!/bin/bash

user_name="${1}"
user_sshkey="${2}"
user_mail="${3}"
user_http_password="${4}"
# Capitalize user_name, e.g. "Zuul CI"
user_fullname="$(tr '[:lower:]' '[:upper:]' <<< ${user_name:0:1})${user_name:1} CI"

# Check if user does not exist yet
user_exists=$(ssh gerrit gerrit ls-members \"Service Users\" | awk '{ print $2 }' | { grep ${user_name} || true; })

if [ -z "$user_exists" ]; then
  echo "$user_sshkey" | ssh gerrit gerrit create-account ${user_name} \
      -g \"Service Users\"                \
      --full-name \"${user_fullname}\"    \
      --ssh-key -
  ssh gerrit gerrit set-account --add-email "${user_mail}" ${user_name}
  ssh gerrit gerrit set-account ${user_name} --http-password "${user_http_password}"
fi