# Secrets rotation

This document provides general guidelines regarding rotating secrets or passwords whose lifecycle is handled by sf-operator.
A table summarizes these secrets and how impactful a leak would be.

1. [Rotating secrets](#rotating-secrets)
1. [Secrets managed by sf-operator]()

## Rotating secrets

The `sf-operator` CLI provides a subcommand that handles rotating **all** the secrets at once for extra security:

```shell
sf-operator rotate-secrets </path/to/cr>
```

Most services need to restart to acknowledge a secret rotation; make sure to plan a service interruption accordingly.

!!! note
    This feature is still under development and some secrets' rotation process is not covered by the CLI.

## Secrets managed by sf-operator

| Secret | Component(s) | covered by `rotate-secrets` | Impact of a secret leak |
|--------|-----------|-----------------------------|-------------------------|
| logserver-keys | zuul | ❌ | **medium** - Gives access to jobs' logs (read/write), can tamper with results |
| config-updater-secrets | config-update job | ❌ | **high** - service account credentials that allow deleting and exec'ing into pods on the deployment's namespace |
| mariadb-root-password | mariadb | ❌ | **low** - access to mariadb component is limited to deployment's namespace. It would allow tampering/deleting builds/buildsets reports |
| nodepool-builder-ssh-key | nodepool-builder | ❌ | **high (dependent)** - grants SSH access to the image builder system if used, with the same privileges as the builder user; tampering of images is possible |
| zookeeper-client-tls | zookeeper, zuul, nodepool-launcher | ❌ | **medium** if zookeeper is accessible to the attacker, **low** if not - grants read/write access to encrypted data in zookeeper |
| zookeeper-server-tls | zookeeper | ❌ | **low** - used to identify zookeeper servers, impersonation is unlikely |
| zuul-auth-secret | zuul | ✅ | **medium** - grants ability to disrupt jobs execution, saturate resources with autoholds |
| zuul-db-connection | zuul, mariadb | ❌ | **low** - access to mariadb component is limited to deployment's namespace, would only allow tampering builds/buildsets reports (but not results) |
| zuul-keystore-password | zuul, zookeeper | ✅ | **high** if zookeeper is accessible to the attacker, **low** if not - allows to decrypt secrets and private keys known to zuul |
| zuul-ssh-key | zuul, nodepool-builder | ❌ | **high** - Grants access to job nodes as the zuul user; allows tampering with jobs' execution and/or results |

