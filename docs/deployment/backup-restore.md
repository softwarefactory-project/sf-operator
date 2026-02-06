# Backup and Restore

The sf-operator CLI provides commands to perform a backup and a restore of a deployment managed by the sf-operator.

The [backup command](../reference/cli/index.md#backup) can be run periodically to perform a backup of a Software Factory deployment.
The command should be coupled with a proper backup system to safely store the backed-up data.

Restoring a backup must be done via the [restore command](../reference/cli/index.md#restore).

## The backup archive

The archive contains:

- Some k8s Secret resources (like the Zuul Keystore Secret and Zuul SSH private key Secret)
- The Zuul SQL database content (history of builds)
- The Zuul projects' private keys (the keys stored in ZooKeeper and used to encrypt/decrypt in-repo Zuul Secrets)
