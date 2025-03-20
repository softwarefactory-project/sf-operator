#!/bin/sh

function getPodName { kubectl get pods -lrun=$1 -o  'jsonpath={.items[0].metadata.name}'; }

kubectl exec -it $(getPodName "$1") sh