#!/bin/sh

set -ex

cat << EOF > /etc/github-ssh-key-updater/config.yaml
gerrit:
  url: "http://gerrit-httpd:8080"
  user: "admin"
  password: "${GERRIT_ADMIN_API_KEY}"
EOF

exec /github-ssh-key-updater-service
