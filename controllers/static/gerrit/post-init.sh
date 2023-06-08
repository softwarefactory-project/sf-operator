#!/bin/bash

set -ex

create_ci_user () {
  local user_name="${1}"
  local user_sshkey="${2}"
  local user_mail="${3}"
  # Capitalize user_name, e.g. "Zuul CI"
  local user_fullname="$(tr '[:lower:]' '[:upper:]' <<< ${user_name:0:1})${user_name:1} CI"

  # Check if user does not exist yet
  local user_exists=$(ssh gerrit gerrit ls-members \"Service Users\" | awk '{ print $2 }' | { grep ${user_name} || true; })

  if [ -z "$user_exists" ]; then
    echo "$user_sshkey" | ssh gerrit gerrit create-account ${user_name} \
        -g \"Service Users\"                \
        --full-name \"${user_fullname}\"    \
        --ssh-key -
    ssh gerrit gerrit set-account --add-email "${user_mail}" ${user_name}
  fi
}


mkdir ~/.ssh
chmod 0700 ~/.ssh

echo "${GERRIT_ADMIN_SSH}" > ~/.ssh/gerrit_admin
chmod 0400 ~/.ssh/gerrit_admin

cat << EOF > ~/.ssh/config
Host gerrit
User admin
Hostname ${GERRIT_SSHD_PORT_29418_TCP_ADDR}
Port ${GERRIT_SSHD_SERVICE_PORT_GERRIT_SSHD}
IdentityFile ~/.ssh/gerrit_admin
StrictHostKeyChecking no
EOF

echo "Ensure we can connect to Gerrit ssh port"
ssh gerrit gerrit version

cat << EOF > ~/.gitconfig
[user]
    name = SF initial configurator
    email = admin@${FQDN}
[gitreview]
    username = admin
[push]
    default = simple
EOF

echo "Set admin account API key (HTTP password)"
ssh gerrit gerrit set-account admin --http-password "${GERRIT_ADMIN_API_KEY}"

echo "Apply ACLs to All-projects"
mkdir ~/All-projects
pushd ~/All-projects
git init .
git remote add origin ssh://gerrit/All-Projects
git fetch origin refs/meta/config:refs/remotes/origin/meta/config
git checkout meta/config
git reset --hard origin/meta/config
gitConfig="git config -f project.config --replace-all "
${gitConfig} capability.accessDatabase "group Administrators"
${gitConfig} access.refs/*.push "group Administrators" ".*group Administrators"
${gitConfig} access.refs/for/*.addPatchSet "group Administrators" "group Administrator"
${gitConfig} access.refs/for/*.addPatchSet "group Project Owners" "group Project Owners"
${gitConfig} access.refs/heads/*.push "+force group Administrators" ".*group Administrators"
${gitConfig} access.refs/heads/*.push "+force group Project Owners" ".*group Project Owners"
${gitConfig} access.refs/heads/*.label-Verified "-2..+2 group Service Users" ".*group Service Users"
${gitConfig} access.refs/heads/*.label-Verified "-2..+2 group Administrators" ".*group Administrators"
${gitConfig} access.refs/heads/*.label-Verified "-2..+2 group Project Owners" ".*group Project Owners"
${gitConfig} access.refs/heads/*.label-Workflow "-1..+1 group Administrators" ".*group Administrators"
${gitConfig} access.refs/heads/*.label-Workflow "-1..+1 group Project Owners" ".*group Project Owners"
${gitConfig} access.refs/heads/*.submit "group Service Users" "group Service Users"
${gitConfig} access.refs/heads/*.rebase "group Administrators" "group Administrators"
${gitConfig} access.refs/heads/*.rebase "group Project Owners" "group Project Owners"
${gitConfig} access.refs/heads/*.rebase "group Service Users" "group Service Users"
${gitConfig} access.refs/heads/*.abandon "group Administrators" "group Administrators"
${gitConfig} access.refs/heads/*.abandon "group Project Owners" "group Project Owners"
${gitConfig} access.refs/meta/config.read "group Registered Users" "group Registered Users"
${gitConfig} access.refs/meta/config.read "group Anonymous Users" "group Anonymous Users"
${gitConfig} access.refs/meta/config.rebase "group Administrators" "group Administrators"
${gitConfig} access.refs/meta/config.rebase "group Project Owners" "group Project Owners"
${gitConfig} access.refs/meta/config.abandon "group Administrators" "group Administrators"
${gitConfig} access.refs/meta/config.abandon "group Project Owners" "group Project Owners"
${gitConfig} access.refs/meta/config.label-Verified "-2..+2 group Administrators" ".*group Administrators"
${gitConfig} access.refs/meta/config.label-Verified "-2..+2 group Project Owners" ".*group Project Owners"
${gitConfig} access.refs/meta/config.label-Workflow "-1..+1 group Administrators" ".*group Administrators"
${gitConfig} access.refs/meta/config.label-Workflow "-1..+1 group Project Owners" ".*group Project Owners"
${gitConfig} access.refs/tags/*.pushTag "+force group Administrators" ".*group Administrators"
${gitConfig} access.refs/tags/*.pushTag "+force group Project Owners" ".*group Project Owners"
${gitConfig} access.refs/tags/*.pushAnnotatedTag "group Administrators" "group Administrators"
${gitConfig} access.refs/tags/*.pushAnnotatedTag "group Project Owners" "group Project Owners"
${gitConfig} access.refs/tags/*.pushSignedTag "group Administrators" "group Administrators"
${gitConfig} access.refs/tags/*.pushSignedTag "group Project Owners" "group Project Owners"
${gitConfig} access.refs/tags/*.forgeAuthor "group Administrators" "group Administrators"
${gitConfig} access.refs/tags/*.forgeAuthor "group Project Owners" "group Project Owners"
${gitConfig} access.refs/tags/*.forgeCommitter "group Administrators" "group Administrators"
${gitConfig} access.refs/tags/*.forgeCommitter "group Project Owners" "group Project Owners"
${gitConfig} access.refs/tags/*.push "group Administrators" "group Administrators"
${gitConfig} access.refs/tags/*.push "group Project Owners" "group Project Owners"
${gitConfig} label.Code-Review.copyAllScoresIfNoCodeChange "true"
${gitConfig} label.Code-Review.value "-2 Do not submit" "-2.*"
${gitConfig} label.Code-Review.value "-1 I would prefer that you didn't submit this" "-1.*"
${gitConfig} label.Code-Review.value "+2 Looks good to me (core reviewer)" "\+2.*"
${gitConfig} label.Verified.value "-2 Fails" "-2.*"
${gitConfig} label.Verified.value "-1 Doesn't seem to work" "-1.*"
${gitConfig} label.Verified.value "0 No score" "0.*"
${gitConfig} label.Verified.value "+1 Works for me" "\+1.*"
${gitConfig} label.Verified.value "+2 Verified" "\+2.*"
${gitConfig} label.Workflow.value "-1 Work in progress" "-1.*"
${gitConfig} label.Workflow.value "0 Ready for reviews" "0.*"
${gitConfig} label.Workflow.value "+1 Approved" "\+1.*"
${gitConfig} plugin.reviewers-by-blame.maxReviewers "5" ".*"
${gitConfig} plugin.reviewers-by-blame.ignoreDrafts "true" ".*"
${gitConfig} plugin.reviewers-by-blame.ignoreSubjectRegEx "'(WIP|DNM)(.*)'" ".*"
git add project.config
git commit -m"Set SF default Gerrit ACLs" && git push origin meta/config:meta/config || true
popd

echo "Ensure Zuul user accounts added into Gerrit"
create_ci_user zuul "${ZUUL_SSH_PUB_KEY}" "zuul@${FQDN}"

echo "Ensure Zuul user accounts API Key added into Gerrit"
ssh gerrit gerrit set-account zuul --http-password "${ZUUL_HTTP_PASSWORD}"

echo "Setup managesf config file"
cat << EOF > ~/config.py
gerrit = {
    'url': 'http://gerrit-httpd:${GERRIT_HTTPD_SERVICE_PORT}/a/',
    'password': '${GERRIT_ADMIN_API_KEY}',
    'host': '${GERRIT_SSHD_SERVICE_HOST}',
    'top_domain': '${FQDN}',
    'ssh_port': 29418,
    'sshkey_priv_path': '~/.ssh/gerrit_admin',
}

resources = {
    'subdir': 'resources',
}

admin = {
    'name': 'admin',
    'email': 'admin@${FQDN}',
}
EOF

# Ensure HTTP access via basic auth for further provisioning
curl --fail -i -u admin:${GERRIT_ADMIN_API_KEY} http://gerrit-httpd:${GERRIT_HTTPD_SERVICE_PORT}/a/accounts/admin

if ! $(ssh gerrit gerrit ls-projects | grep -q "^config$"); then
  echo "Create config repository and related groups"
  cat << EOF > ~/prev.yaml
resources: {}
EOF
  cat << EOF > ~/new.yaml
resources:
  acls:
    config-acl:
      file: |
        [access "refs/*"]
          read = group config-core
          owner = group config-ptl
        [access "refs/heads/*"]
          label-Code-Review = -2..+2 group config-core
          label-Code-Review = -2..+2 group config-ptl
          label-Verified = -2..+2 group config-ptl
          label-Workflow = -1..+1 group config-core
          label-Workflow = -1..+1 group config-ptl
          label-Workflow = -1..+0 group Registered Users
          rebase = group config-core
          abandon = group config-core
          submit = group config-ptl
          read = group config-core
          read = group Registered Users
        [access "refs/meta/config"]
          read = group config-core
          read = group Registered Users
        [receive]
          requireChangeId = true
        [submit]
          mergeContent = false
          action = fast forward only
      groups:
        - config-core
        - config-ptl
      name: config-acl
  groups:
    config-core:
      description: Team core for the config repo
      members: []
      name: config-core
    config-ptl:
      description: Team lead for the config repo
      members:
        - "admin@${FQDN}"
      name: config-ptl
  repos:
    config:
      acl: config-acl
      description: Config repository
      name: config
EOF
  managesf-resources --managesf-config ~/config.py \
    --cache-dir ~/ direct-apply --new-yaml ~/new.yaml --prev-yaml ~/prev.yaml
else
  echo "config repository already exists"
fi
