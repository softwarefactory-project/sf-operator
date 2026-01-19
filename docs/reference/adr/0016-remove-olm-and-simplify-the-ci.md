---
status: proposed
date: 2025-07-30
---

# Remove OLM and simplify the CI

## Context and Problem Statement

The `sf-operator` can be run in two primary ways:

1.  **Standalone CLI Mode:** A one-shot command-line execution that applies a configuration. This is our primary mode for both development and production. It allows for rapid iteration, debugging, and patching in production environments.
2.  **Operator/Bundle Mode:** A long-running pod that watches a Custom Resource, providing a more conventional cloud-native UX where changes are applied via `kubectl apply`.

The installation of the operator for the second mode can be done manually (the "Bundle" approach) or via the Operator Lifecycle Manager (OLM). In practice, OLM was not practical in our deployment environments, so proven rather useless and complex for our use cases.
Lastly, the sf-operator was bootstrapped with an old version of the operator-sdk, to generate a lot of boilerplate to manage the bundle, which is already outdated and that will requires a significant amount of work to update.


## Considered Options

*   **Deprecate the Entire Operator Pattern:** Focus on the standalone CLI mode.
*   **Deprecate OLM:** Deprecate only the OLM installation method but keep the Operator/Bundle mode integration.
*   **Continue Supporting All Installation Methods:** Do not change the project.

## Decision Outcome

Chosen option: "**Deprecate the Entire Operator Pattern**", because it comes out best (see below).


### Consequences

* Good, because we simplify the CI by removing the sf-operator-olm-rhel and sf-operator-publish-olm-bundle-image job. We also simplify the playbook by removing the "olm mode" checks.
* Good, because we can simplify the Makefile and remove the kubebuilder ACL annotation.
* Good, because we can remove the rbac and the operator deployment resources in the `config` repository.
* Good, because we clean the project and be in a better position to upgrade the operator-sdk boilerplate when we want to re-introduce the bundle mode.
* Good, because the CI will run faster.
* Bad, because we loose the ability to deploy the bundle and Software Factory using only `kubectl`.
* Neutral, because we keep the main mechanic based the operator-framework/sdk, which will allow us to re-enable the operator partern/bundle mode later if there is a clear need.
* Bad, because we have to refactor the CI infrastructure.

## Pros and Cons of the Options

### Continue Supporting All Installation Methods

* Good, because the sf-operator is the most flexible that way.
* Bad, because it creates a maintainance burden for the Bundle that we don't use in production.

### Deprecate OLM

* Good, because the operator bundle is still supported and this is a recommended way to deploy software on OpenShift.
* Good, because we can still remove the catalog publication and the OLM integration test.
* Bad, because we can't really simplify the CI since we need to take into account the custom apply resource.
* Bad, because we keep the outdated dependency on the operator-sdk boilerplate.


## More Information

To implement this decision, the following actions are mandatory:

1.  **Rewrite User Documentation:** Overhaul the documentation to establish the Standalone CLI as the primary production method.
2.  **Remove OLM-Specific Artifacts:** Remove all code, configuration, and build artifacts related to OLM packaging (CSVs, catalog sources, etc.).
3.  **Update CI/CD:** All CI jobs related to OLM deployment and upgrade testing must be removed. CI must continue to validate the Standalone CLI mode, and a new testshould be added to ensure the upgrade path between N-1 and N via the Standalone CLI.
