#!/bin/sh

ansible-playbook playbooks/main.yaml \
    -e "hostname=localhost" \
    -e 'install_requirements=false' \
    -e '{"zuul":{"project":{"src_dir": ".."}}}' \
    $*
