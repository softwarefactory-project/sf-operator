FROM quay.io/operator-framework/ansible-operator:v0.6.0
USER root
RUN yum install -y git
USER 1000

COPY watches.yaml ${HOME}/watches.yaml

COPY ansible/gerrit.yaml ${HOME}/gerrit.yaml
COPY ansible/software-factory.yaml ${HOME}/software-factory.yaml
COPY ansible/group_vars/ ${HOME}/group_vars/
COPY ansible/roles/ ${HOME}/roles/

COPY build/uid_entrypoint.sh /uid_entrypoint
