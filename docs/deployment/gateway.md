# Gateway

The httpd-based service gateway provides a single entrypoint for all web services by setting up a reverse proxy.

You can directly serve the gateway for outbound traffic, for example with a route like this one:

```yaml
kind: Route
apiVersion: route.openshift.io/v1
metadata:
  name: sf-gateway-route
  namespace: my-sf
spec:
  host: my-sf.com
  path: /
  to:
    kind: Service
    name: gateway
    weight: 100
  port:
    targetPort: 8080
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
  wildcardPolicy: None
```

## Reverse proxy configuration

The following paths are configured by default:

| web path | service |
|-----------|---------|
| /logs     | [logserver](./logserver.md) |
| /nodepool/builds | [nodepool image builds logs](./nodepool.md) |
| /nodepool/api | [nodepool launcher API](./nodepool.md) |
| /zuul | [zuul Web](./zuul.md) |
| /zuul-capacity | zuul-capacity |
| /weeder | zuul-weeder |
| /logjuicer | logjuicer |
| /codesearch | hound code search |

## Extending the gateway

The gateway comes with a very minimal configuration that should work for most use cases.
You may however want to extend this configuration for simple things like custom MIME types
definitions or serving static content; or for more complex things like SSO authentication.

For "complex" features, it is advised to use an extra reverse proxy pod in front of this gateway, for which you will have complete control over its configuration.

For "simpler" features, in particular ones that do not require secrets, you can use the `gateway.extraConfigurationConfigMap` and `gateway.extraStaticFilesConfigMap` settings on the Software Factory CRD. These settings point at config maps that define extra configuration files to be loaded by the httpd service, and static files to mount in /var/www/html, respectively.

For example, to put Software Factory in maintenance mode, you can use the following configmaps:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: maintenance-httpd-config
data:
  # the default config starts with 99- to be loaded last
  00-maintenance.conf: |
    <VirtualHost *:8080>
        DocumentRoot "/var/www/html"
        ErrorDocument 503 /maintenance.html
        <IfModule mod_rewrite.c>
            RewriteEngine on
            RewriteCond %{REQUEST_URI} !=/maintenance.html
            RewriteRule ^.*$ - [R=503,L]
        </IfModule>
    </VirtualHost>
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: maintenance-static-files
data:
  maintenance.html: |
    Site under maintenance
```

create the config maps, edit your Software Factory resource:

```yaml
apiVersion: sf.softwarefactory-project.io/v1
kind: SoftwareFactory
metadata:
  name: my-sf
spec:
  [...]
  gateway:
    extraConfigurationConfigMap: maintenance-httpd-config
    extraStaticFilesConfigMap: maintenance-static-files
```

After redeploying SF, the gateway will serve the maintenance page on all requests and return the error code HTTP 503.