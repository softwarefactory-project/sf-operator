#!/bin/sh

set -ex

# The script expects by default to find the 'nodepool.yaml' file in
# the config repository. However the same for nodepool-builder the script
# must find the 'nodepool-builder.yaml' in the config repo. Thus, this script
# can be parameterized via the NODEPOOL_CONFIG_FILE environment variable.
NODEPOOL_CONFIG_FILE="${NODEPOOL_CONFIG_FILE:-nodepool.yaml}"

# Generate the default tenants configuration file
cat << EOF > ~/nodepool.yaml
---
webapp:
  port: 8006
zookeeper-servers:
  - host: zookeeper
    port: 2281
zookeeper-tls:
  ca: /tls/client/ca.crt
  cert: /tls/client/tls.crt
  key: /tls/client/tls.key
# images-dir is mandatory key for nodepool-builder process
images-dir: /var/lib/nodepool/dib
EOF

if [ "$CONFIG_REPO_SET" == "TRUE" ]; then
  # A config repository has been set

  REF=$1
  REF=${REF:-origin/master}

  # Clone or fetch config repository
  if [ -d ~/config/.git ]; then
    pushd ~/config
    git remote remove origin
    git remote add origin ${CONFIG_REPO_BASE_URL}/${CONFIG_REPO_NAME}
    git fetch origin
    git reset --hard $REF
    popd
  else
    pushd ~/
    git clone ${CONFIG_REPO_BASE_URL}/${CONFIG_REPO_NAME} config
    popd
  fi

  # Append the config repo provided config file to the default one
  if [ -f ~/config/nodepool/${NODEPOOL_CONFIG_FILE} ]; then
    cat ~/config/nodepool/${NODEPOOL_CONFIG_FILE} >> ~/nodepool.yaml
  fi

fi

echo "Generated nodepool config:"
echo
cat ~/nodepool.yaml
cp ~/nodepool.yaml /etc/nodepool/nodepool.yaml
