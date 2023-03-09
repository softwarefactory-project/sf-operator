#!/bin/sh

ansible-playbook playbooks/test-microshift.yaml  --extra-vars "hostname=localhost" --extra-vars 'install_requirements=false' --extra-vars '{"zuul":{"project":{"src_dir": ".."}}}' $*
