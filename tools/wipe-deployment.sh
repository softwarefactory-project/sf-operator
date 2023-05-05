#!/bin/sh

ansible-playbook playbooks/wipe.yaml \
    -e "hostname=localhost" \
    -e 'remote_os_host=false'