#!/bin/bash

SF_OPERATOR_DIR=${SF_OPERATOR_DIR:-.}
MICROSHIFT_HOST=${MICROSHIFT_HOST:-}
MICROSHIFT_KUBECONFIG=${MICROSHIFT_KUBECONFIG:-"$HOME/.kube/config"}
NODEPOOL_KUBECONFIG=${NODEPOOL_KUBECONFIG:-"$HOME/.kube/nodepool.config"}

if [ -z "$MICROSHIFT_HOST" ]; then
    MICROSHIFT_HOST=$(ip route get 1.2.3.4 | awk '{print $7}' | head -n1)
fi

kubectl apply -f ./tools/nodepool-microshift-service-account.yaml
TOKEN=$(kubectl --namespace nodepool get secrets nodepool-sa-secret -o jsonpath="{ .data.token }" | base64 -d)
CA=$(grep certificate-authority-data $MICROSHIFT_KUBECONFIG | awk '{print $2}')

cat << EOF > "$NODEPOOL_KUBECONFIG"
---
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: $CA
    server: https://$MICROSHIFT_HOST:6443
  name: microshift
contexts:
- context:
    cluster: microshift
    namespace: nodepool
    user: nodepool
  name: microshift
current-context: microshift
kind: Config
preferences: {}
users:
- name: nodepool
  user:
    token: $TOKEN
EOF

kubectl create secret generic nodepool-kubeconfig --from-file=config=$NODEPOOL_KUBECONFIG
