#!/bin/bash

REPO_NAME=$1

[ ! -n "${REPO_NAME}" ] && {
  echo "Usage: create-repo.sh <repo_name>"
  exit 1
}

cat << EOF > ~/prev.yaml
resources: {}
EOF

cat << EOF > ~/new.yaml
resources:
  acls:
    ${REPO_NAME}-acl:
      file: |
        [access "refs/*"]
          read = group ${REPO_NAME}-core
          owner = group ${REPO_NAME}-ptl
        [access "refs/heads/*"]
          label-Code-Review = -2..+2 group ${REPO_NAME}-core
          label-Code-Review = -2..+2 group ${REPO_NAME}-ptl
          label-Verified = -2..+2 group ${REPO_NAME}-ptl
          label-Workflow = -1..+1 group ${REPO_NAME}-core
          label-Workflow = -1..+1 group ${REPO_NAME}-ptl
          label-Workflow = -1..+0 group Registered Users
          rebase = group ${REPO_NAME}-core
          abandon = group ${REPO_NAME}-core
          submit = group ${REPO_NAME}-ptl
          read = group ${REPO_NAME}-core
          read = group Registered Users
        [access "refs/meta/config"]
          read = group ${REPO_NAME}-core
          read = group Registered Users
        [receive]
          requireChangeId = true
        [submit]
          mergeContent = false
          action = fast forward only
      groups:
        - ${REPO_NAME}-core
        - ${REPO_NAME}-ptl
      name: ${REPO_NAME}-acl
  groups:
    ${REPO_NAME}-core:
      description: Team core for the ${REPO_NAME} repo
      members: []
      name: ${REPO_NAME}-core
    ${REPO_NAME}-ptl:
      description: Team lead for the ${REPO_NAME} repo
      members:
        - "admin@${FQDN}"
      name: ${REPO_NAME}-ptl
  repos:
    ${REPO_NAME}:
      acl: ${REPO_NAME}-acl
      description: ${REPO_NAME} repository
      name: ${REPO_NAME}
EOF

managesf-resources --managesf-config /etc/managesf/config.py \
    --cache-dir ~/ direct-apply --new-yaml ~/new.yaml --prev-yaml ~/prev.yaml