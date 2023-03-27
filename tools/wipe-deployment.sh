#!/bin/sh

set -ex

# Remove the "my-sf" deployment
kubectl -n default delete SoftwareFactory "${1:-my-sf}"

# Remove the Persistent Volume Claims (PVs and data are deleted as we use topolvm)
kubectl -n default delete pvc --all
