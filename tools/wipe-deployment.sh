#!/bin/sh

set -ex

oc -n default delete all --all --now

for resource in certificates ClusterIssuers issuers certificaterequests secrets pvc configmaps deployments pods services ingress;
do
  oc -n default delete $resource --all;
done

# Delete all content in the PV
oc get pv | cut -f 1 -d ' ' | xargs oc delete pv
