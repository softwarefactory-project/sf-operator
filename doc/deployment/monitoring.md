# Monitoring

Here you will find information about what monitoring is available on services deployed with SF-Operator.

## Table of Contents

1. [Concepts](#concepts)
1. [Accessing the metrics](#accessing-the-metrics)
1. [Statsd](#statsd)
1. [Predefined alerts](#predefined-alerts)

## Concepts

SF-Operator use the [prometheus-operator](https://prometheus-operator.dev/) to expose and collect service metrics.
SF-Operator will automatically create [PodMonitors](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#podmonitor) for the following services:

* Log Server
* [Nodepool](./nodepool.md)
* [Zuul](./zuul.md)

| Service | Statsd metrics | Prometheus metrics |
|---------|--------|-------|
| Log Server | ❌ | ✅ |
| Nodepool | ✅ | ✅ |
| Zuul | ✅ | ✅ |

The `PodMonitors` are set with the label key `sf-monitoring` (and a value equal to the monitored service name); that key can be used for filtering metrics.

You can list the PodMonitors this way:

```sh
kubectl get podmonitors
```

The `Log server` service runs the [Node Exporter](https://prometheus.io/docs/guides/node-exporter/) process as a sidecar container as well, in order to expose disk space-related metrics.

For services that expose statsd metrics, a sidecar container running [Statsd Exporter](https://github.com/prometheus/statsd_exporter)
is added to the service pod, so that these metrics can be consumed by a Prometheus instance.

## Accessing the metrics

If [enabled in your cluster](https://docs.openshift.com/container-platform/4.13/monitoring/enabling-monitoring-for-user-defined-projects.html#enabling-monitoring-for-user-defined-projects), metrics will automatically
be collected by the cluster-wide Prometheus instance. Check with your cluster admin about getting access to your metrics.

If this feature isn't enabled in your cluster, you will need to deploy your own Prometheus instance to collect the metrics on your own.
To do so, you can either:

* Follow the [CLI documentation](./../cli/index.md#prometheus) to deploy a standalone Prometheus instance
* Follow the [prometheus-operator's documentation](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md#deploying-prometheus) to deploy it on your own

In the latter case, you will need to set the proper `PodMonitorSelector` in the Prometheus instance's manifest:

```yaml
  # assuming Prometheus is deployed in the same namespace as SF
  podMonitorNamespaceSelector: {}
  podMonitorSelector:
    matchExpressions:
    - key: sf-monitoring
      operator: Exists
```

## Statsd

### Statsd Exporter mappings

Statsd Exporter sidecars are preconfigured to map every statsd metric issued by Zuul and Nodepool into prometheus-compatible metrics.
You can find the mappings definitions [here (Nodepool)](./../../controllers/static/nodepool/statsd_mapping.yaml) and [here (Zuul)](./../../controllers/static/zuul/statsd_mapping.yaml).

### Forwarding

It is possible to use the `relayAddress` property in a SoftwareFactory CRD to define a different statsd collector for Zuul and Nodepool, for example an external graphite instance.

## Predefined alerts

SF-Operator defines some metrics-related alert rules for the deployed services.

> The alert rules are defined for Prometheus. Handling these alerts (typically sending out notifications) requires another service called [AlertManager](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/alerting.md). How to manage AlertManager is out of scope for this documentation.
You may need to [configure](https://docs.openshift.com/container-platform/4.13/monitoring/managing-alerts.html#sending-notifications-to-external-systems_managing-alerts) or
[install](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/alerting.md) an
AlertManager instance on your cluster,
or configure Prometheus to forward alerts to an external AlertManager instance.

The following alerting rules are created automatically at deployment time:

| Alert name | Severity  | Description |
|---------|------|------------------|
| `OutOfDiskNow` | critical | The Log server has less than 10% free storage space left |
| `OutOfDiskInThreeDays` | warning | Assuming a linear trend, the Log server's storage space will fill up in less than three days |
| `ConfigUpdateFailureInPostPipeline` | critical | A `config-update` job failed in the `post` pipeline, meaning a configuration change was not applied properly to the Software Factory deployment's services |
| `NotEnoughExecutors` | warning | Lack of resources is throttling performance in the last hour; in that case some jobs are waiting for an available executor to run on |
| `NotEnoughMergers` | warning | Lack of resources is throttling performance in the last hour; in that case some merge jobs are waiting for an available merger to run on |
| `NotEnoughTestNodes` | warning | Lack of resources is throttling performance in the last hour; in that case Nodepool could not fulfill node requests |
| `DIBImageBuildFailure` | warning | the disk-image-builder service (DIB) failed to build an image |
| `HighFailedStateRate` | critical | Triggers when more than 5% of nodes on a provider are in failed state over a period of one hour |
| `HighNodeLaunchErrorRate` | critical | Triggers when more than 5% of node launch events end in an error state over a period of one hour |
| `HighOpenStackAPIError5xxRate` | critical | Triggers when more than 5% of API calls on OpenStack return a status code of 5xx (server-side error) over a period of 15 minutes |

If [statsd metrics prefixes are set](https://docs.openstack.org/openstacksdk/latest/user/guides/stats.html) for clouds defined in Nodepool's `clouds.yaml`, SF-Operator will also create the following alert
for each cloud with a set prefix:

| Alert name | Severity  | Description |
|---------|------|------------------|
| `HighOpenStackAPIError5xxRate_<CLOUD NAME>` | critical | Triggers when more than 5% of API calls on cloud <CLOUD NAME> return a status code of 5xx (server-side error) over a period of 15 minutes |

Note that these alerts are generic and might not be relevant to your deployment's specificities.
For instance, it may be normal to hit the `NotEnoughTestNodes` alert if resource quotas are in place
on your Nodepool providers.

You are encouraged to [create your own alerts](https://prometheus-operator.dev/docs/user-guides/alerting/#deploying-prometheus-rules), using these ones as a base.