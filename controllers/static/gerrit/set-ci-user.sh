#!/bin/sh
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.

set -x
set -e

USER_NAME="${1}"
USER_SSHKEY="${2}"
USER_MAIL="${3}"
# Capitalize user_name, e.g. "Zuul CI"
USER_FULLNAME="$(tr '[:lower:]' '[:upper:]' <<< ${USER_NAME:0:1})${USER_NAME:1} CI"

# Check if user does not exist yet
USER_EXISTS=$(ssh gerrit gerrit ls-members \"Service Users\" | awk '{ print $2 }' | { grep ${USER_NAME} || true; })

if [ -z "$USER_EXISTS" ]; then
    echo "$USER_SSHKEY" | ssh gerrit gerrit create-account ${USER_NAME} \
        -g \"Service Users\"                \
        --email "${USER_MAIL}"              \
        --full-name \"${USER_FULLNAME}\"    \
        --ssh-key -
fi