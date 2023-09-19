# Running the test suites locally

Before running the tests, make sure the operator was started in another terminal:

```sh
go run ./main.go --namespace sf
```

Tests run in the [project's CI](https://softwarefactory-project.io/zuul/t/local/buildsets?project=software-factory%2Fsf-operator) can also be run locally using the `sfconfig` CLI:

```sh
./tools/sfconfig runTests
```

This command is a wrapper on top of `ansible-playbook` to run the same Ansible playbook
than the CI. This includes steps to deploy the operator if needed.

`runTests` performs a build and installation of the `OLM package` of the `sf-operator` prior to
running the validation test suite.

If you want to only run the test suite part (the functional tests only, [assuming a SoftwareFactory instance was already deployed](./getting_started.md)), then you can use the `--test-only` option:

```sh
./tools/sfconfig runTests --test-only
```

The command accepts extra Ansible parameters to be passed to `ansible-playbook` command.
For instance to override the default `microshift_host` variable:

```sh
./tools/sfconfig runTests --extra-var "microshift_host=my-microshift"
```

To get more Ansible output logs, you can use the `verbose (-v)` or `debug (-vvv)` parameter.
For example:

```sh
/tools/sfconfig runTests -v
```

To run the upgrade sf-operator test scenario, run the command below:

```sh
/tools/sfconfig runTests --upgrade
```

To fetch the test suite artifacts (service logs, operator logs, etc) locally, run:

```sh
./tools/fetch-artifacts.sh
```

The artifacts will be available in the `/tmp/sf-operator-artifacts/` directory.
