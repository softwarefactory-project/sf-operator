# sf-operator test suite

## Usage

From the project root:

- Run the tests:
```
go test -v ./tests/... -args --ginkgo.v
```

- Run a specific test:
```
go test -v ./tests/... -args --ginkgo.v --ginkgo.focus "Secret Rotations"
```

- List the available tests:
```
go test -v ./tests/... -args --ginkgo.v --ginkgo.dry-run
```

- Delete test resources (might be necessary when test breaks the deployment):
```
kubectl delete cm sf-standalone-owner
```

## Contribute

Tests are written using:
- https://onsi.github.io/ginkgo/#spec-subjects-it
- https://onsi.github.io/gomega/#working-with-strings-json-and-yaml

The test library and entrypoint is defined in the main_test.go package.
