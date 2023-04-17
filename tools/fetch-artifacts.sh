#!/bin/sh

ansible-playbook playbooks/post.yaml \
    -e "hostname=localhost" \
    -e "output_logs_dir=/tmp/sf-operator-artifacts"
