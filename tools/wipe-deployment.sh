#!/bin/sh

set -ex

# Remove the "my-sf" deployment
kubectl -n default get SoftwareFactory my-sf && \
  kubectl -n default delete SoftwareFactory my-sf

# Keep ca-cert (perhaps not needed ?)
for secret in $(kubectl get secrets -o json | jq -r '.items[].metadata.name' | grep -v "ca-cert"); do
  kubectl -n default delete secrets $secret
done;

# Remove the Persistent Volume Claims (PVs and data are deleted as we use topolvm)
kubectl -n default delete pvc --all;