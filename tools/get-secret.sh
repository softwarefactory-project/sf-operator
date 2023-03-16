#!/bin/sh

echo $(oc get secret $1 -o yaml | grep "$1:" | awk '{print $2}') | base64 -d
echo ""
