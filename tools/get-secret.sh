#!/bin/sh

secret_name=$1

if [ -n "$2" ]; then
    matcher=$2
else
    matcher=$1
fi

echo $(oc get secret ${secret_name} -o json | jq -r ".data.\"${matcher}\"") | base64 -d
echo ""
