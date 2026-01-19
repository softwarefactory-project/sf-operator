#!/usr/bin/env bash

# Copyright (C) 2023 Red Hat
# SPDX-License-Identifier: Apache-2.0

REPO_ROOT=DOC_ROOT="${DOC_ROOT:-$(cd "$(dirname "$0")/../" && pwd)}"
API_ROOT="${API_ROOT:-$(cd "$(dirname "$0")/../api/v1/" && pwd)}"
DOC_ROOT="${DOC_ROOT:-$(cd "$(dirname "$0")/../docs/" && pwd)}"

tmpdir="$(mktemp -d)"
docstmpdir="$(mktemp -d)"

cleanup() {
  echo "Cleaning up temporary GOPATH"
  rm -rf "${docstmpdir}"
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

GOMODCACHE="$(go env GOMODCACHE)"
export GOMODCACHE
export GOPATH="${tmpdir}/go"
GOROOT="$(go env GOROOT)"
export GOROOT
GOBIN="${tmpdir}/bin"
export GOBIN

mkdir -p "${GOPATH}/src/github.com/softwarefactory-project"
GITDIR="${GOPATH}/src/github.com/softwarefactory-project/sf-operator"
git clone --depth=1 "file://${REPO_ROOT}" "$GITDIR"
pushd "$GITDIR"
# go mod vendor

go get github.com/elastic/crd-ref-docs@v0.0.10
go install github.com/elastic/crd-ref-docs

mkdir -p "${docstmpdir}/apidocs/"
${GOBIN}/crd-ref-docs \
    --config "${DOC_ROOT}/_apidoc/config.yaml" \
    --source-path "./api" \
    --output-path "${docstmpdir}/apidocs/index.md" \
    --templates-dir "${DOC_ROOT}/_apidoc/templates" \
    --renderer=markdown

popd

# Add any post-processing here
mv "${docstmpdir}/apidocs/index.md" "${DOC_ROOT}/reference/api/index.md"

# Add stub sections for types that are referenced but not generated
cat >> "${DOC_ROOT}/reference/api/index.md" << 'EOF'

#### SoftwareFactoryStatus

_Type alias for:_ _[BaseStatus](#basestatus)_

SoftwareFactoryStatus defines the observed state of SoftwareFactory. It is a type alias for BaseStatus with no additional fields.

_Appears in:_
- [SoftwareFactory](#softwarefactory)

#### SecretRef

_This type is defined but not currently used in the public API._

SecretRef selects a key of a secret in the pod's namespace.

_Appears in:_
- [Secret](#secret)

| Field | Description | Default Value |
| --- | --- | --- |
| `secretKeyRef` _[Secret](#secret)_ | Selects a key of a secret in the pod's namespace | - |

#### LetsEncryptSpec

_This type is defined but not currently implemented in the operator._

LetsEncryptSpec specifies the Let's Encrypt server configuration. This feature is planned but not yet implemented.

| Field | Description | Default Value |
| --- | --- | --- |
| `server` _[LEServer](#leserver)_ | Specify the Let's Encrypt server. Valid values are: "staging", "prod" | staging |

EOF
