# Set up TLS on service routes

## Table of Contents

1. [Using existing X509 certificates](#using-x509-certificates)
1. [Using Let's Encrypt](#using-lets-encrypt)

By default, a SF deployment comes with a self-signed Certificate Authority delivered by the [cert-manager operator](https://cert-manager.io/), and HTTP services in the deployment get certificates from this CA to enable secure ingress with TLS. The cert-manager operator also **simplifies certificates lifecycle managagement by handling renewals and routes reconfigurations automatically**.

Currently, the list of concerned HTTP services is:

- logserver HTTP endpoint
- nodepool web API
- zuul-web

While this is good enough for testing, the cert-manager operator also allows you to integrate your deployment with an existing, trusted Certificate Authority, or even use [Let's Encrypt](https://letsencrypt.org/).

## Using X509 certificates

The operator watches specific `Secrets` in the `SoftwareFactory` Custom Resources namespace.
When those secrets' data hold a Certificate, Key and CA Certificate (following a specific scheme) then
the sf-operator is able to reconfigure the corresponding service `Route`'s TLS to use the TLS material
stored in the secret.

The `sfconfig` command can be used to configure these secrets.

> The `create-service-ssl-secret` subcommand will validate the SSL certificate/key before updating the `Secret`.

The example below updates the `Secret` for the `logserver` service. The `SoftwareFactory` Custom 
Resource will pass into a "non Ready" state until the `Route` is reconfigured.
Once `Ready`, the `Route` will present the new Certificate.

```sh
./tools/sfconfig create-service-ssl-secret \
    --sf-service-ca /tmp/ssl/localCA.pem \
    --sf-service-key /tmp/ssl/ssl.key \
    --sf-service-cert /tmp/ssl/ssl.crt \
    --sf-service-name logserver
```

Allowed `sf-service-name` values are:

  - logserver
  - zuul
  - nodepool
  - gerrit (if deployed with the CLI)

## Using Let's Encrypt

The SF Operator offers an option to request Certificates from `Let's Encrypt` using the `ACME http01`
challenge. All DNS names exposed by the `Routes` must be publicly resolvable.

> This overrides any custom X509 certificates that might have been set following the steps above.

1. test your deployment with Let's Encrypt's staging server:

```sh
kubectl edit sf my-sf
[...]
spec:
  letsEncrypt:
    server: "staging"
[...]
```

The `SoftwareFactory` CR will pass into a "non Ready" state until all `Challenges` are resolved
and all the `Routes` are reconfigured.

2. Set the server to `prod` to use the Let's Encrypt production server:

```sh
kubectl edit sf my-sf
[...]
spec:
  letsEncrypt:
    server: "prod"
[...]
```

Once the `SoftwareFactory` Custom Resource is ready, your services are using certificates issued by Let's Encrypt.