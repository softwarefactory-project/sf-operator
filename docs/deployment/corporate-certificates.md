# Add corporate CA certificates to the CA trust chain

Some components like Zuul and Nodepool may need to communicate with corporate services via HTTPS.
When such corporate services expose a certificate signed by a corporate Certificate Authority, the CA certificate must be part of the CA trust chain of the component's container.

sf-operator eases the installation of additional CA certificates into Zuul and Nodepool containers via a dedicated ConfigMap resource.

The dedicated ConfigMap resource must be named `corporate-ca-certs`.
The ConfigMap's content will be mounted into `/usr/share/pki/ca-trust-source/` and processed by the `update-ca-trust` command at container startup.

When the ConfigMap is changed, the controller automatically recognizes it and restarts the corresponding pods.
