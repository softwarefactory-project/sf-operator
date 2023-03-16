#!/bin/sh

function getPodName { oc get pods -lrun=$1 -o  'jsonpath={.items[0].metadata.name}'; }

oc exec -it $(getPodName "$1") sh