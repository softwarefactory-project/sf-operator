# dhall-operator library

> This is imported from https://github.com/TristanCacqueray/dhall-operator
> Ideally this should live in a separate project,
> but it might be easier to start by vendoring it

## Objects

This library contains:

* types: top level is Application.dhall.
* schemas: are types with default values.
* functions: convenient functions to manipulate the types.
* deploy: the main deployment functions.

## Example

The examples/Demo.dhall is an Application type and it can be used to generate:

```console
# A localhost playbook
$ dhall-to-yaml <<< '(./deploy/Ansible.dhall).Localhost ./examples/Demo.dhall'
- hosts: localhost
  tasks:
    - command: "podman create --name demo-postgres --network=host docker.io/library/postgres:12.1"
[...]

# A distributed playbook
$ dhall-to-yaml <<< '(./deploy/Ansible.dhall).Distributed ./examples/Demo.dhall'

- hosts: postgres
  tasks:
    - command: "podman create --name demo-postgres --network=host docker.io/library/postgres:12.1"
      name: "Create container"

# A podman compose script
$ dhall text <<< '(./deploy/Podman.dhall).RenderCommands ./examples/Demo.dhall'
#!/bin/bash -ex
podman pod create --name demo

podman run --pod demo --name demo-dns --detach --add-host=postgres:127.0.0.1 [...]

# A kubernetes objects collection
$ dhall-to-yaml --omit-empty <<< './deploy/Kubernetes.dhall ./examples/Demo.dhall'
apiVersion: v1
items:
  - apiVersion: v1
    kind: Service
    metadata:
      labels:
        app.kubernetes.io/component: postgres
        app.kubernetes.io/instance: demo
        app.kubernetes.io/name: demo
        app.kubernetes.io/part-of: app
      name: postgres
    spec:
      ports:
        - name: pg
          port: 5432
          protocol: TCP
          targetPort: pg
[...]
```
