#!/bin/bash

NS=$1
DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

if [ -z "$NS" ]; then
    NS=$(KUBECONFIG=~/.kube/config ; kubectl config view | grep namespace | cut -f2 -d: | xargs)
fi

if ! [ -f "${DIR}/mysf.yaml" ]; then
    echo -e "\nReplacing FQDN..."
    sed "s/fqdn: \"sftests.com\"/fqdn: \"${NS}.sftests.com\"/g" config/samples/sf_v1_softwarefactory.yaml > "${DIR}/mysf.yaml"
fi

echo -e "\nDeploying Software Factory..."
go run ./main.go --cr "${DIR}/mysf.yaml" --namespace "${NS}"
