#!/bin/sh

set -ex

MY_NS=$(kubectl config view -o jsonpath='{.contexts[].context.namespace}')

if [ -z "${MY_NS}" ]; then
    echo "Unable to find a context namespace in user kube config"
    exit 1
fi

kubectl -n $MY_NS delete all --all --now

for resource in certificates ClusterIssuers issuers certificaterequests secrets pvc configmaps deployments pods services ingress;
do
  kubectl -n $MY_NS delete $resource --all;
done