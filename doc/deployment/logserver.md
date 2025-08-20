# Logserver: Dedicated Log Management

This section provides information about the log management service deployed with the Software Factory Operator.

Logserver, a dedicated server developed for the Software Factory project, handles Zuul log deletion.
When Zuul executes a pipeline, it generates logs.
Logserver's `purgelogs` component automatically deletes these logs based on their age.

Logserver within the Software Factory Operator is deployed as a StatefulSet resource consisting of the following containers:

| Name | Image |
|---------|--------------------------|
| logserver | registry.access.redhat.com/ubi8/httpd-24:1-284.1696531168 |
| logserver-sshd | quay.io/software-factory/sshd:0.1-3 |
| purgelogs | quay.io/software-factory/purgelogs:0.2.4-1 |
| logserver-nodeexporter | quay.io/prometheus/node-exporter:v1.6.1 |

## logserver
A dedicated HTTP server that exposes logs through a web interface.
## logserver-sshd
A container that provides an SSH CLI specifically for fetching Zuul build logs.
## purgelogs
Runs a background process that continuously checks the log's age against a threshold defined in the Software Factory Operator Custom Resource (CR).
## logserver-nodeexporter
Exposes metrics about the Logserver pod, enabling the monitoring of its resource utilization and performance.
