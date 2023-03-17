#!/bin/sh

set -ex

kubectl -n default delete all --all --now

for resource in certificates ClusterIssuers issuers certificaterequests secrets pvc configmaps deployments pods services ingress;
do
  kubectl -n default delete $resource --all;
done

# Delete all content in the PV
kubectl get pv | cut -f 1 -d ' ' | xargs kubectl delete pv
