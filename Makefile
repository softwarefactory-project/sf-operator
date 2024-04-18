# Within the CI pipelines the VERSION number is auto managed
# For an upgrade job the version number is generated to high N value
# For the publish job we use the git tag name as the version number
VERSION ?= 0.0.1

# Image URL to use all building/pushing image targets
BASE_REPO ?= quay.io/software-factory/sf-operator
IMG ?= $(BASE_REPO):v$(VERSION)

BUNDLE_REPO ?= $(BASE_REPO)-bundle
BUNDLE_IMG ?= $(BUNDLE_REPO):v$(VERSION)

CATALOG_REPO ?= $(BASE_REPO)-catalog
CATALOG_IMG ?= $(CATALOG_REPO):latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
# NOTE: MicroShift 4.13 got kubeAPI 1.26.
# More info: https://docs.openshift.com/container-platform/4.13/release_notes/ocp-4-13-release-notes.html#ocp-4-13-about-this-release
ENVTEST_K8S_VERSION = 1.26
CERT_MANAGER_VERSION = v1.12.3

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

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

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

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

##@ Build

.PHONY: setenv
setenv:
	go env -w GOSUMDB="sum.golang.org" GOPROXY="https://proxy.golang.org,direct"

.PHONY: build
build: setenv generate fmt vet sc build-api-doc ## Build manager binary.
	go build -o bin/sf-operator main.go

.PHONY: build-api-doc
build-api-doc: # Build the API documentation.
	./tools/build-api-doc.sh

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

.PHONY: operator-build
operator-build: ## Build podman image with the manager.
	podman build -t ${IMG} .

.PHONY: operator-push
operator-push: ## Push podman image with the manager.
	podman push ${IMG}

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(LOCALBIN)/operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(LOCALBIN)/operator-sdk generate bundle $(BUNDLE_GEN_FLAGS)
	$(LOCALBIN)/operator-sdk bundle validate ./bundle --verbose

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	podman build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	podman push $(BUNDLE_IMG)

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: install-cmctl
install-cmctl: ## Install the cert-manager cmctl CLI
	@bash -c "mkdir -p bin; test -f bin/cmctl || { curl -sSL -o cmctl.tar.gz https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cmctl-linux-amd64.tar.gz && tar xzf cmctl.tar.gz && mv cmctl ./bin; rm cmctl.tar.gz; }"

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
MKDOCS ?= $(LOCALBIN)/mkdocs/bin/mkdocs

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0
OPERATOR_SDK_VERSION ?= 1.32.0
STATICCHECK_VERSION ?= 2023.1.6
MKDOCS_VERSION ?= 1.5.3
SETUP_ENVTEST_VERSION ?= v0.0.0-20240320141353-395cfc7486e6

.PHONY: mkdocs
mkdocs: $(MKDOCS) ## Install material for mkdocs locally if necessary
$(MKDOCS): $(LOCALBIN)
	( test -f $(LOCALBIN)/mkdocs/bin/mkdocs && [[ "$(shell $(LOCALBIN)/mkdocs/bin/mkdocs -V)" =~ "$(MKDOCS_VERSION)" ]] ) || ( python -m venv $(LOCALBIN)/mkdocs && $(LOCALBIN)/mkdocs/bin/pip install --upgrade -r mkdocs-requirements.txt )

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/kustomize/${KUSTOMIZE_VERSION}/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	( test -f $(LOCALBIN)/kustomize && [[ "$(shell $(LOCALBIN)/kustomize version --short)" =~ "$(KUSTOMIZE_VERSION)" ]] ) || ( touch $(LOCALBIN)/kustomize && rm -f $(LOCALBIN)/kustomize && curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN) )

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	( test -f $(LOCALBIN)/controller-gen && [[ "$(shell $(LOCALBIN)/controller-gen --version)" =~ "$(CONTROLLER_TOOLS_VERSION)" ]]  ) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION)

.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK)
$(OPERATOR_SDK): $(LOCALBIN)
	(test -f $(LOCALBIN)/operator-sdk && [[ "$(shell $(LOCALBIN)/operator-sdk version)" =~ "$(OPERATOR_SDK_VERSION)" ]] ) || (curl -o $(LOCALBIN)/operator-sdk -sSL https://github.com/operator-framework/operator-sdk/releases/download/v${OPERATOR_SDK_VERSION}/operator-sdk_linux_amd64 && chmod +x $(LOCALBIN)/operator-sdk )

.PHONY: staticcheck
staticcheck:
	(test -f $(LOCALBIN)/staticcheck && [[ "$(shell $(LOCALBIN)/staticcheck --version)" =~ "$(STATICCHECK_VERSION)" ]] ) || GOBIN=$(LOCALBIN) go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	mkdir -p $(GOBIN)
	test -L $(GOBIN)/staticcheck || ln -s $(LOCALBIN)/staticcheck $(GOBIN)/staticcheck

# Cataloge
CATALOG_DIR=sf-operator-catalog
CATALOG_FILE=$(CATALOG_DIR)/catalog.yaml
OPM=$(LOCALBIN)/opm
CHANNEL=preview
AVAILTAGS = $(shell skopeo list-tags docker://$(BUNDLE_REPO) | jq -r '.Tags[]' | grep -v latest )
OPERATOR_REGISTRY_VERSION = v1.28.0

.PHONY: install-opm
install-opm: ## Install the cert-manager cmctl CLI
	@bash -c "mkdir -p $(LOCALBIN); test -f $(OPM) || (curl -sSL https://github.com/operator-framework/operator-registry/releases/download/${OPERATOR_REGISTRY_VERSION}/linux-amd64-opm -o $(OPM) && chmod +x $(OPM) )"

.PHONY: opm-dir-gen
opm-dir-gen: install-opm
	@bash -c "mkdir -p $(CATALOG_DIR)"

opm-files-gen: opm-dir-gen
	test -f $(CATALOG_DIR).Dockerfile || $(OPM) generate dockerfile $(CATALOG_DIR)
	$(OPM) init sf-operator --default-channel=$(CHANNEL) --description=./README.md --output yaml > $(CATALOG_FILE)
ifdef AVAILTAGS
	$(foreach version,$(AVAILTAGS),$(OPM) render $(BUNDLE_REPO):$(version) --output=yaml >> $(CATALOG_FILE);)
else
	$(error There are no available tags)
endif

.PHONY: schema-operator
schema-operator: opm-files-gen
	@printf "\n\
	---\n\
	schema: olm.channel\n\
	package: sf-operator\n\
	name: $(CHANNEL)\n\
	entries:\
	">> $(CATALOG_FILE)

schema-entries:
	$(call entries_write_fn)

schema-channel: schema-operator schema-entries

.PHONY: opm
opm: schema-channel
	$(OPM) validate $(CATALOG_DIR)

opm-build:
	podman build -f $(CATALOG_DIR).Dockerfile -t $(CATALOG_IMG)

opm-push:
	podman push $(CATALOG_IMG)

.PHONY: opm-clean
clean-opm:
	@bash -c "rm -r $(CATALOG_DIR) $(CATALOG_DIR).Dockerfile"

# function with one argument $(function_name,arg1)
entry_name_fn = \
$(shell printf "\n  - name: sf-operator.$(1)\
">> $(CATALOG_FILE))

# function with one argument $(function_name,arg1)
entry_replace_fn = \
$(shell printf "\n    replaces: sf-operator.$(1)\
">> $(CATALOG_FILE))

# function with two arguments $(function_name,arg1,arg2)
define entry_fn
	$(call entry_name_fn,$(1))
	$(if $(2), $(call entry_replace_fn,$(2)) )
endef

# function with no arguments
define entries_write_fn
	$(foreach version,$(AVAILTAGS),$(call entry_fn,$(version),$(prevversion)) $(eval prevversion=$(version)))
endef
