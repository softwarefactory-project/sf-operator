#!/bin/bash

# This script will create a Kubernetes Dashboards service

# NOTE: The clusterRole policy has been changed as recommended in:
# https://github.com/kubernetes/dashboard/issues/4179

curl -LO https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml

sed -i 's/namespace: kubernetes-dashboard/namespace: sf/g' recommended.yaml
sed -i 's/namespace=kubernetes-dashboard/namespace=sf/g' recommended.yaml
sed -i 's/port: 443/port: 8443/g' recommended.yaml

kubectl apply -f recommended.yaml

oc create route passthrough kubernetes-dashboard --service=kubernetes-dashboard --port=8443 --hostname=dashboards.sfop.dev
kubectl adm policy add-scc-to-user privileged -z kubernetes-dashboard

kubectl apply -f - <<EOF
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
rules:
  # Allow Metrics Scraper to get metrics from the Metrics server
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods", "nodes"]
    verbs: ["get", "list", "watch"]

  # Other resources
  - apiGroups: [""]
    resources: ["nodes", "namespaces", "pods", "serviceaccounts", "services", "configmaps", "endpoints", "persistentvolumeclaims", "replicationcontrollers", "replicationcontrollers/scale", "persistentvolumeclaims", "persistentvolumes", "bindings", "events", "limitranges", "namespaces/status", "pods/log", "pods/status", "replicationcontrollers/status", "resourcequotas", "resourcequotas/status", "secrets"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["apps"]
    resources: ["daemonsets", "deployments", "deployments.apps", "deployments/scale", "replicasets", "replicasets/scale", "statefulsets"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["batch"]
    resources: ["cronjobs", "jobs", "jobs.batch"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["extensions"]
    resources: ["daemonsets", "deployments", "deployments/scale", "networkpolicies", "replicasets", "replicasets/scale", "replicationcontrollers/scale"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses", "ingressclasses", "networkpolicies"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["storage.k8s.io"]
    resources: ["storageclasses", "volumeattachments"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["clusterrolebindings", "clusterroles", "roles", "rolebindings"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["apiextensions.k8s.io"]
    resources: ["customresourcedefinitions.apiextensions.k8s.io"]
    verbs: ["get", "list", "watch"]
EOF

# Print token to login into: dashboards.sfop.dev
# Remember to add dashboards.sfop.dev into /etc/hosts!
kubectl create token kubernetes-dashboard

# Now use the token to reach:
curl https://dashboards.sfop.dev/#/workloads?namespace=sf
