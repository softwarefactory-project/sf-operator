# Zuul

Here you will find information about managing the Zuul service when deployed with the SF Operator.
It does not replace [Zuul's documentation](https://zuul-ci.org/docs/zuul/latest/),
but addresses specificities and idiosyncrasies induced by deploying Zuul with the SF Operator.

## Table of Contents

1. [Zuul-Client](#zuul-client)

## Zuul-Client

The `sfconfig` CLI can act as a "proxy" of sorts for the `zuul-client` CLI, by directly calling  `zuul-client` from a running Zuul web pod. For example, to read zuul-client's help message:

```bash
./tools/sfconfig zuul-client -h
```