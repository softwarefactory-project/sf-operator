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
A container that provides SSH access for uploading Zuul build logs.
## purgelogs
Runs a background process that continuously checks the log's age against a threshold defined in the Software Factory Operator Custom Resource (CR).
## logserver-nodeexporter
Exposes metrics about the Logserver pod, enabling the monitoring of its resource utilization and performance.

# SSH Access and Key Management

Logserver provides SSH access for uploading build logs. Authentication is handled through SSH public keys configured via Kubernetes secrets.

## SSH Key Secrets

### logserver-uploader-keys

The primary secret containing SSH keys for log uploading.
This secret is created automatically when Software Factory is deployed for the first time.

The public key from this secret is automatically added to the logserver's `authorized_keys` file, allowing the corresponding private key holder to upload logs via SSH.

### logserver-uploader-spare-keys

This secret is created automatically when Software Factory is deployed for the first time and supports zero-downtime SSH key rotation. During key rotation periods, both `logserver-uploader-keys` and `logserver-uploader-spare-keys` keys are authorized simultaneously, allowing long-running jobs to complete with their original keys.

This secret will be automatically recreated by the operator if deleted, ensuring it's always available for rotation workflows.
See the [secrets rotation workflow](./secrets_rotation.md#logserver-keys-rotation) for step-by-step instructions.

## Dynamic Key Synchronization

The Software Factory Operator automatically reconciles SSH keys without requiring pod restarts:

- Keys are synchronized directly to the running logserver pod
- Changes to `logserver-uploader-keys` or `logserver-uploader-spare-keys` are detected and applied dynamically
- The `authorized_keys` file is updated atomically to prevent service disruption
- If the optional `logserver-uploader-spare-keys` secret is not present, only the primary key is used

This approach ensures continuous availability during key rotation and eliminates the need for manual pod restarts.

## Integration with Zuul Jobs

Zuul jobs upload build logs to the logserver via SSH using the private key from the `logserver-uploader-keys` secret. This private key is encrypted as a Zuul secret and made available to jobs during execution, enabling automated log publishing to the logserver's web interface.

The private key can be encrypted using the `zuul-client` tool and stored in the project's secret definition. For implementation details, see:

- **Zuul documentation**: The standard [upload-logs role](https://zuul-ci.org/docs/zuul-jobs/latest/general-roles.html#role-upload-logs) from zuul-jobs
