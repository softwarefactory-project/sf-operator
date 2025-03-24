#!/bin/bash

function fail {
    echo "ERROR: $@"1>&2
    exit 1
}

if [ $# -ne 1 ]
then
    echo -e "\n$0 [host]"
    echo -e "$0 localhost\n"
    fail "Missing host"
fi

HOST=$1

./hack/setup-ansible.sh

ansible-playbook -e "hostname=${HOST}" -e "create_ramdisk=false" ./playbooks/install-minikube.yaml
[ $? -ne 0 ] && fail "Installation of Minikube failed"
ansible-playbook -e "hostname=${HOST}" ./playbooks/prepare-minikube.yaml
[ $? -ne 0 ] && fail "Configuration of Minikube failed"
exit 0
