#!/bin/bash

set -ex

env
cd ${HOME}

mkdir .ssh
chmod 0700 .ssh

echo "${GERRIT_ADMIN_SSH}" > .ssh/gerrit_admin
chmod 0400 .ssh/gerrit_admin

cat << EOF > .ssh/config
Host gerrit
User admin
Hostname ${GERRIT_SSHD_PORT_29418_TCP_ADDR}
Port ${GERRIT_SSHD_SERVICE_PORT_GERRIT_SSHD}
IdentityFile /root/.ssh/gerrit_admin
StrictHostKeyChecking no
EOF

echo "Ensure we can connect to Gerrit ssh port"
ssh gerrit gerrit version

cat << EOF > .gitconfig
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
mkdir All-projects && cd All-projects
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
cd -

echo "Ensure CI user accounts added into Gerrit"
for CI_USER_ENV in $(env | grep CI_USER_SSH_ | cut -d "=" -f1); do
  name=$(echo ${CI_USER_ENV} | sed 's/CI_USER_SSH_//')
  /bin/bash /entry/set-ci-user.sh $name "${!CI_USER_ENV}" "${name}@${FQDN}"
done

echo "Ensure CI user accounts API Key added into Gerrit"
for CI_USER_ENV in $(env | grep CI_USER_API_ | cut -d "=" -f1); do
  name=$(echo ${CI_USER_ENV} | sed 's/CI_USER_API_//')
  ssh gerrit gerrit set-account ${name} --http-password "${!CI_USER_ENV}"
done

if [ "${HAS_CONFIG_REPOSITORY}" == "true" ]; then
  echo "Setup managesf config file"
  mkdir -p /etc/managesf
  cat << EOF > /etc/managesf/config.py
gerrit = {
    'url': 'http://gerrit-httpd:${GERRIT_HTTPD_SERVICE_PORT}/a/',
    'password': '${GERRIT_ADMIN_API_KEY}',
    'host': '${GERRIT_SSHD_SERVICE_HOST}',
    'top_domain': '${FQDN}',
    'ssh_port': 29418,
    'sshkey_priv_path': '/root/.ssh/gerrit_admin',
}

resources = {
    'subdir': 'resources',
}

admin = {
    'name': 'admin',
    'email': 'admin@${FQDN}',
}
EOF
  cat /etc/managesf/config.py

  # Ensure HTTP access via basic auth for further provisioning
  curl --fail -i -u admin:${GERRIT_ADMIN_API_KEY} http://gerrit-httpd:${GERRIT_HTTPD_SERVICE_PORT}/a/accounts/admin

  if ! $(ssh gerrit gerrit ls-projects | grep -q "^config$"); then
    echo "Create config repository and related groups"
    cat << EOF > prev.yaml
resources: {}
EOF
    dhall-to-yaml-ng --output \
      new.yaml <<< "(/entry/resources.dhall).renderInitialResources \"${FQDN}\""
    managesf-resources direct-apply --new-yaml new.yaml --prev-yaml prev.yaml
  else
    echo "config repository already exists"
  fi
else
  echo "config repository not hosted on Gerrit"
fi