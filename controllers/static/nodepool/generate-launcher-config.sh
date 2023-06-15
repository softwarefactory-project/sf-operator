#!/bin/sh

set -ex

export HOME=/tmp

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
labels:
  - name: f38-nonroot
providers:
  - name: microshiftLocal
    driver: openshiftpods
    context: microshift
    pools:
      # NOTE: name is a name of the namespace
      # https://github.com/softwarefactory-project/sf-config/blob/master/ansible/roles/sf-repos/files/config/nodepool/openshift.yaml#L30
      # https://zuul-ci.org/docs/nodepool/latest/openshift-pods.html
      - name: nodepool
        labels:
          # FIXME: move the image to quay.io
          - name: f38-nonroot
            image: localhost/local:f38-nonroot
            python-path: /bin/python3
EOF

if [ "$CONFIG_REPO_SET" == "TRUE" ]; then
  # A config repository has been set

  REF=$1
  REF=${REF:-origin/master}

  # Clone or fetch config repository
  if [ -d ~/${CONFIG_REPO_NAME}/.git ]; then
    pushd ~/${CONFIG_REPO_NAME}
    git remote remove origin
    git remote add origin ${CONFIG_REPO_BASE_URL}/${CONFIG_REPO_NAME}
    git fetch origin
    git reset --hard $REF
    popd
  else
    pushd ~/
    git clone ${CONFIG_REPO_BASE_URL}/${CONFIG_REPO_NAME}
    popd
  fi

  # Append the config repo provided config file to the default one
  if [ -f ~/${CONFIG_REPO_NAME}/nodepool/nodepool.yaml ]; then
    cat ~/${CONFIG_REPO_NAME}/nodepool/nodepool.yaml >> ~/nodepool.yaml
  fi

fi

cp ~/nodepool.yaml /etc/nodepool/nodepool.yaml
