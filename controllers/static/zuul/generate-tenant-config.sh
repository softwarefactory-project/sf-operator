#!/bin/sh

set -ex

export HOME=/var/lib/zuul

# Generate the default tenants configuration file
cat << EOF > ~/main.yaml
- tenant:
    max-job-timeout: 10800
    name: internal
    report-build-page: true
    exclude-unprotected-branches: true
    source:
      git-server:
        config-projects:
        - system-config
      opendev.org:
        untrusted-projects:
        - zuul/zuul-jobs
EOF

if [ "$CONFIG_REPO_SET" == "TRUE" ]; then
  # A config repository has been set

  # config-update usage context required a specific git ref
  REF=$1

  /usr/local/bin/fetch-config-repo.sh $REF

  # Ensure the config repo enabled into the tenants config
  cat << EOF >> ~/main.yaml
      ${CONFIG_REPO_CONNECTION_NAME}:
        config-projects:
        - ${CONFIG_REPO_NAME}
EOF

  # Append the config repo provided tenant file to the default one
  if [ -f ~/config/zuul/main.yaml ]; then
    cat ~/config/zuul/main.yaml >> ~/main.yaml
  fi

fi

echo "Generated tenants config:"
echo
cat ~/main.yaml