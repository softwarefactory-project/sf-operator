# Custom Resource Definitions

This document gives details about the Custom Resource Definitions (CRDs) that the SF-Operator installs on a cluster.

The specs are constantly evolving during alpha development and should be considered unstable, but they are the ultimate source of truth for documentation about their properties.

1. [SoftwareFactory](#softwarefactory)

## SoftwareFactory

This custom resource describes a Software Factory instance.

```yaml
--8<-- "config/crd/bases/sf.softwarefactory-project.io_softwarefactories.yaml"
```
