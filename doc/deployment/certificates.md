# Set up TLS on service routes

## Table of Contents

1. [Using a trusted Certificate Authority](#using-a-trusted-certificate-authority)
1. [Using Let's Encrypt](#using-lets-encrypt)

By default, a SF deployment comes with a self-signed Certificate Authority delivered by the Ingress manager, and HTTPS services in the deployment get certificates from this CA to enable secure ingress with TLS.

Currently, the list of concerned HTTPS services is:

- logserver HTTP endpoint
- nodepool web API and nodepool build's logs
- zuul-web

While this is good enough for testing, the sf-operator allows you to integrate your deployment with an existing, trusted Certificate Authority, or even use [Let's Encrypt](https://letsencrypt.org/).

## Using a trusted Certificate Authority

The operator watches a `Secret` named `sf-ssl-cert` in the `SoftwareFactory` Custom Resources namespace.
When this `Secret`'s data hold a Certificate, Key and CA Certificate (following a specific scheme) then
the sf-operator is able to reconfigure all managed `Route`'s TLS to use the TLS material stored in the secret.

> Make sure that the certificate `CN` matches the `fqdn` setting of the `SoftwareFactory` Custom Resource.

The `sfconfig` command can be used to configure these secrets.

> The `create-service-ssl-secret` subcommand will validate the SSL certificate/key before updating the `Secret`.

```sh
./tools/sfconfig create-service-ssl-secret \
    --sf-service-ca /tmp/ssl/localCA.pem \
    --sf-service-key /tmp/ssl/ssl.key \
    --sf-service-cert /tmp/ssl/ssl.crt
```

Alternatively, the `sf-ssl-cert` `Secret` resource can be managed without the `sfconfig` helper by
following this Secret's layout:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sf-ssl-cert
data:
  CA: "<CA Trust Chain>"
  crt: "<Your Server Certificate>"
  key: "<Your server Certificate's key>"
```

The `SoftwareFactory` Custom Resource will pass into a "non Ready" state until reconfiguration is completed.
Once `Ready`, all managed `Route` will present the new certificate.


## Using Let's Encrypt

The SF Operator offers an option to request a certificate from `Let's Encrypt` using the `ACME http01`
challenge. The deployment `FQDN` must be publicly resolvable.

> This overrides the custom X509 certificates that might have been set following the [steps above](#using-a-trusted-certificate-authority).

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

Once the `SoftwareFactory` Custom Resource is ready, all managed `Route` will present the new certificate
issued by Let's Encrypt.

