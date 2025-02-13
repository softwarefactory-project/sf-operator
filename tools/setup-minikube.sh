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

if ! which ansible-playbook 2>/dev/null
then
        DIST=$(awk '/^ID=/' /etc/os-release | sed 's/^ID=//' | tr '[:upper:]' '[:lower:]' | tr -d '[:punct:]')
        if [[ "${DIST}" == "centos" ]] || [[ "${DIST}" == "fedora" ]]
        then
                sudo dnf install -y ansible-core
        else
                fail "Distribution ${DIST} not yet supported"
        fi
fi

ansible-playbook -e "hostname=${HOST}" ./playbooks/install-minikube.yaml
[ $? -ne 0 ] && fail "Installation of Minikube failed"
ansible-playbook -e "hostname=${HOST}" ./playbooks/prepare-minikube.yaml
[ $? -ne 0 ] && fail "Configuration of Minikube failed"
exit 0
