## Tool Versions
CONTROLLER_TOOLS_VERSION = v0.18.0
SETUP_ENVTEST_VERSION = release-0.22
STATICCHECK_VERSION = 2025.1.1
MKDOCS_VERSION = 1.6.0
ZUUL_CLIENT_VERSION = 10.0.0

LDFLAGS = -ldflags="-X github.com/softwarefactory-project/sf-operator/controllers/libs/utils.version=$(VERSION)"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build test

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Cleanup the local env
	@rm -Rf $(LOCALBIN)/*

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: sc
sc: staticcheck ## Run staticcheck checks https://staticcheck.dev/docs/
	$(LOCALBIN)/staticcheck ./...

.PHONY: doc-serve
doc-serve: mkdocs
	$(LOCALBIN)/mkdocs/bin/mkdocs serve

.PHONY: doc-build
doc-build: mkdocs build-api-doc ## Build documentation with MkDocs to local _site without publishing
	$(MKDOCS) build --site-dir ./_site

.PHONY: doc-check
doc-check: mkdocs build-api-doc ## Build documentation and fail on warnings
	$(MKDOCS) build --site-dir ./_site --strict

.PHONY: test
test: manifests generate fmt vet envtest vendor-crds ## Run tests.
	CGO_ENABLED=1 KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test -race ./controllers/... -coverprofile cover.out

.PHONY: integration-test
integration-tests: ## Run golang integration tests.
	go test -timeout 0 -v ./tests/... -args --ginkgo.v --ginkgo.no-color

##@ Build

.PHONY: setenv
setenv:
	go env -w GOSUMDB="sum.golang.org" GOPROXY="https://proxy.golang.org,direct"

.PHONY: build
build: setenv manifests generate fmt vet sc build-api-doc ## Build manager binary.
	go build $(LDFLAGS) -o bin/sf-operator main.go

.PHONY: build-api-doc
build-api-doc: # Build the API documentation.
	./hack/build-api-doc.sh

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run $(LDFLAGS) main.go

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
MKDOCS ?= $(LOCALBIN)/mkdocs/bin/mkdocs
ZC ?= $(LOCALBIN)/zc/bin/zuul-client

.PHONY: mkdocs
mkdocs: $(MKDOCS) ## Install material for mkdocs locally if necessary
$(MKDOCS): $(LOCALBIN)
	( test -f $(LOCALBIN)/mkdocs/bin/mkdocs && [[ "$(shell $(LOCALBIN)/mkdocs/bin/mkdocs -V)" =~ "$(MKDOCS_VERSION)" ]] ) || ( python -m venv $(LOCALBIN)/mkdocs && $(LOCALBIN)/mkdocs/bin/pip install --upgrade -r mkdocs-requirements.txt )

.PHONY: zuul-client
zuul-client: $(ZC) ## Install zuul-client locally if necessary
$(ZC): $(LOCALBIN)
	( test -f $(LOCALBIN)/zc/bin/zuul-client && [[ "$(shell $(LOCALBIN)/zc/bin/zuul-client --version)" =~ "$(ZUUL_CLIENT_VERSION)" ]] ) || ( python -m venv $(LOCALBIN)/zc && $(LOCALBIN)/zc/bin/pip install zuul-client )

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	( test -f $(LOCALBIN)/controller-gen && [[ "$(shell $(LOCALBIN)/controller-gen --version)" =~ "$(CONTROLLER_TOOLS_VERSION)" ]]  ) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION)

.PHONY: staticcheck
staticcheck:
	(test -f $(LOCALBIN)/staticcheck && [[ "$(shell $(LOCALBIN)/staticcheck --version)" =~ "$(STATICCHECK_VERSION)" ]] ) || GOBIN=$(LOCALBIN) go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	mkdir -p $(GOBIN)
	test -L $(GOBIN)/staticcheck || ln -s $(LOCALBIN)/staticcheck $(GOBIN)/staticcheck

# TODO: remove this when the last stable version doesn't use the prometheus operator
.PHONY: vendor-crds
vendor-crds:
	@mkdir -p config/crd/vendor/
	@(test -f config/crd/vendor/monitoring.yaml || curl -Lo config/crd/vendor/monitoring.yaml https://github.com/prometheus-operator/prometheus-operator/releases/download/v0.82.2/stripped-down-crds.yaml)

.PHONY: render-dhall-schemas
render-dhall-schemas:
	@rm -f schemas/*
	cabal run
