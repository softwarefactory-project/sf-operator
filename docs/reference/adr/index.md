---
title: About
---

# Decisions

This directory contains decision records for sf-operator.

For new ADRs, please use the template below as a boilerplate:

??? abstract "ADR template"

    ```markdown title="template.md"
    --8<-- "doc/reference/adr/adr-template.md"
    ```

More information on MADR is available at <https://adr.github.io/madr/>.
General information about architectural decision records is available at <https://adr.github.io/>.


1. [Use Markdown for ADRs](./0000-use-markdown-any-decision-records.md)
1. [Operator Configuration](./0001-operator-config.md)
1. [Zuul System config](./0002-zuul-system-config.md)
1. [Config update workflow - base system](./0003-config-update.md)
1. [Expose main.yaml to Zuul scheduler](./0004-zuul-main.md)
1. [Command-line tool to set up and manage sf-operator deployment](./0005-ops-tooling.md)
1. [Operator and Operand Metrics Collection](./0006-monitoring.md)
1. [Edge certificates management](./0007-edge-cert.md)
1. [Config check and update jobs implementation](./0008-config-jobs.md)
1. [Database Agnosticity for SF Deployments](./0009-database-agnosticity.md)
1. [Usage of the upstream zuul-operator](./0010-zuul-operator-usage.md)
1. [Nodepool Builder](./0011-nodepool-builder.md)
1. [CLI overhaul](./0012-CLI-overhaul.md)
1. [Route management](./0015-route-handling.md)
1. [Remove OLM and simplify the CI](./0016-remove-olm-and-simplify-the-ci.md)
1. [Project code hosting change](./0017-sf-operator-code-hosting.md)