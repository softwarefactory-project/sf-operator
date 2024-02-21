# Running the test suites locally

Tests run in the [project's CI](https://zuul.microshift.softwarefactory-project.io/zuul/t/sf/buildsets) can also be run locally using the [`sf-operator dev run-tests` CLI subcommand](./../reference/cli/index.md#run-tests). (1)
{ .annotate }

1. This command is a wrapper on top of `ansible-playbook` to run the same Ansible playbook
   than the CI. This includes steps to deploy the operator if needed.

The command accepts extra Ansible parameters to be passed to `ansible-playbook` command.

For instance to override the default `microshift_host` variable:

```sh
sf-operator dev run-tests TEST_NAME --extra-var "microshift_host=my-microshift"
```

To get more Ansible output logs, you can use the `verbose (--v)` or `debug (--vvv)` parameter.
For example:

```sh
sf-operator dev run-tests TEST_NAME --v
```

## Available test suites

### The OLM validation test

The `OLM` test (similar to `sf-operator-olm` in the Zuul CI) performs a build and
installation of the `OLM package` of the `sf-operator` prior to running the validation
test suite.

To perform this test, run the following command:

```sh
sf-operator dev run-tests olm
```

### The OLM upgrade validation test

The `OLM upgrade` test (similar to `sf-operator-upgrade` in the Zuul CI) performs the installation via `OLM` of the current published version of the operator then
build the current local version and upgrade the currently deployed version.
Finally, runs the validation test suite.

To run the upgrade sf-operator test scenario, run the following command:

```sh
sf-operator dev run-tests upgrade
```

### The standalone validation test

The `standalone` tests (1)  (similar to `sf-operator-dev-multinode` in the Zuul CI) performs
a standalone deployment and run the validation test suite.
{ .annotate }

1. This is fastest way to run the test suite when iterating on the development of the `sf-operator`.

```sh
sf-operator dev run-tests standalone
```

## Fetching test artifacts

To fetch the test suite artifacts (service logs, operator logs, etc) locally, run:

```sh
./tools/fetch-artifacts.sh
```

The artifacts will be available in the `/tmp/sf-operator-artifacts/` directory.
