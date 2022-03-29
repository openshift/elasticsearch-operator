CURPATH=$(PWD)

export GOBIN=$(CURDIR)/bin
export PATH:=$(GOBIN):$(PATH)

include .bingo/Variables.mk

export GOROOT=$(shell go env GOROOT)
export GOFLAGS=
export GO111MODULE=on

export LOGGING_VERSION=5.5

export APP_NAME=elasticsearch-operator

export ARTIFACT_DIR?=./tmp/artifacts
export JUNIT_REPORT_OUTPUT_DIR=$(ARTIFACT_DIR)/junit
COVERAGE_DIR=$(ARTIFACT_DIR)/coverage

IMAGE_TAG?=127.0.0.1:5000/openshift/origin-$(APP_NAME):latest
APP_REPO=github.com/openshift/$(APP_NAME)
KUBECONFIG?=$(HOME)/.kube/config
MAIN_PKG=main.go
RUN_LOG?=elasticsearch-operator.log
RUN_PID?=elasticsearch-operator.pid
LOGGING_IMAGE_STREAM?=stable
OPERATOR_NAMESPACE=openshift-operators-redhat
DEPLOYMENT_NAMESPACE?=openshift-logging
REPLICAS?=0
OS_NAME=$(shell uname -s | tr '[:upper:]' '[:lower:]')

GO_FILES     := $(shell find . -type f -name '*.go')
BUNDLE_FILES := $(shell find bundle/ -type f)
OTHER_FILES  := $(shell find files/ -type f)

.PHONY: all build clean fmt generate gobindir help run test-e2e test-unit

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

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-32s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

all: build

artifactdir:
	@mkdir -p $(ARTIFACT_DIR)
.PHONY: artifactdir

coveragedir: artifactdir
	@mkdir -p $(COVERAGE_DIR)
.PHONY: junitreportdir

junitreportdir: artifactdir
	@mkdir -p $(JUNIT_REPORT_OUTPUT_DIR)
.PHONY: junitreportdir

gobindir:
	@mkdir -p $(GOBIN)

GEN_TIMESTAMP=.zz_generate_timestamp
generate: $(OPERATOR_SDK) $(CONTROLLER_GEN) $(GEN_TIMESTAMP) ## Generate APIs and CustomResourceDefinition objects.
$(GEN_TIMESTAMP): $(shell find apis -name '*.go')
	@$(CONTROLLER_GEN) object paths="./apis/..."
	@$(CONTROLLER_GEN) crd:crdVersions=v1 rbac:roleName=elasticsearch-operator paths="./..." output:crd:artifacts:config=config/crd/bases
	@$(MAKE) fmt
	@touch $@

regenerate: $(OPERATOR_SDK) $(CONTROLLER_GEN)  ## Force generate CustomResourceDefinition objects.
	@rm -f $(GEN_TIMESTAMP)
	@$(MAKE) generate

# Generate bundle manifests and metadata, then validate generated files.
BUNDLE_VERSION?=$(LOGGING_VERSION).0
# Options for 'bundle-build'
BUNDLE_CHANNELS := --channels=stable,stable-${LOGGING_VERSION}
BUNDLE_DEFAULT_CHANNEL := --default-channel=stable
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

bundle: regenerate $(KUSTOMIZE) ## Generate operator bundle.
	$(OPERATOR_SDK) generate kustomize manifests -q
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(BUNDLE_VERSION) $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK) bundle validate ./bundle
.PHONY: bundle

build: ## Build the operator binary.
	@go build -o $(GOBIN)/elasticsearch-operator $(MAIN_PKG)

clean: ## Clean tmp, _output dirs and go cache/testcache.
	@rm -rf bin tmp _output
	go clean -cache -testcache ./...

fmt: $(GOFUMPORTS) ## Run gofumpt against code.
	@$(GOFUMPORTS) -l -w $(shell find internal apis controllers test version -name '*.go') ./*.go

lint: $(GOLANGCI_LINT) fmt lint-prom lint-dockerfile ## Run golangci-lint against code.
	@GOLANGCI_LINT_CACHE="$(CURDIR)/.cache" $(GOLANGCI_LINT) run -c golangci.yaml

lint-prom: $(PROMTOOL) ## Run promtool check against recording rules and alerts.
	@$(PROMTOOL) check rules ./files/prometheus_recording_rules.yml
	@$(PROMTOOL) check rules ./files/prometheus_alerts.yml

gen-dockerfiles: ## Generate dockerfile from midstream contents.
	./hack/generate-dockerfile-from-midstream > Dockerfile && \
	./hack/generate-dockerfile-from-midstream Dockerfile.in dev-meta.yaml > Dockerfile.dev
.PHONY: gen-dockerfiles

lint-dockerfile: ## Lint for upstream/downstream dockerfile changes.
	@hack/lint-dockerfile
.PHONY: lint-dockerfile

image: .output/image ## Build operator container image.
.output/image: gen-dockerfiles $(GO_FILES) $(BUNDLE_FILES) $(OTHER_FILES)
	podman build -f Dockerfile.dev -t $(IMAGE_TAG) .
	@touch $@

##@ Testing

test-unit: $(GO_JUNIT_REPORT) coveragedir junitreportdir test-unit-prom ## Run unit tests.
	@set -o pipefail && \
		go test -race -coverprofile=$(COVERAGE_DIR)/test-unit.cov ./internal/... ./apis/... ./controllers/... ./. 2>&1 | \
		tee /dev/stderr | \
		$(GO_JUNIT_REPORT) > $(JUNIT_REPORT_OUTPUT_DIR)/junit.xml
	@grep -v 'zz_generated\.' $(COVERAGE_DIR)/test-unit.cov > $(COVERAGE_DIR)/nogen.cov
	@go tool cover -html=$(COVERAGE_DIR)/nogen.cov -o $(COVERAGE_DIR)/test-unit-coverage.html
	@go tool cover -func=$(COVERAGE_DIR)/nogen.cov | tail -n 1

test-unit-prom: $(PROMTOOL) ## Run prometheus unit tests.
	@$(PROMTOOL) test rules ./test/files/prometheus-unit-tests/test.yml

test-e2e-upgrade: ## Run e2e upgrate tests.
	@hack/testing-olm-upgrade/test-upgrade-n-1-n.sh
.PHONY: test-e2e-upgrade

# Run e2e upgrade tests on upstream CI.
test-e2e-upgrade-ci:
	@DO_SETUP="false" hack/testing-olm-upgrade/test-upgrade-n-1-n.sh
.PHONY: test-e2e-upgrade-ci

# to use these targets, ensure the following env vars are set:
# either each IMAGE env var:
# IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY
# IMAGE_ELASTICSEARCH_OPERATOR
# IMAGE_ELASTICSEARCH6
# IMAGE_ELASTICSEARCH_PROXY
# IMAGE_LOGGING_KIBANA6
# IMAGE_OAUTH_PROXY
#
# You must also set:
# ELASTICSEARCH_OPERATOR_NAMESPACE (Default: openshift-operators-redhat)
RANDOM_SUFFIX:=$(shell echo $$RANDOM)
TEST_NAMESPACE?="e2e-test-${RANDOM_SUFFIX}"
test-e2e-olm: DEPLOYMENT_NAMESPACE="${TEST_NAMESPACE}" ## Run e2e tests.
test-e2e-olm: $(GO_JUNIT_REPORT) $(JUNITMERGE) $(JUNITREPORT) junitreportdir
	TEST_NAMESPACE=${TEST_NAMESPACE} hack/test-e2e.sh
	echo "Completed test-e2e"
	$(JUNITMERGE) $$(find $$JUNIT_REPORT_OUTPUT_DIR -iname "*.xml") > $(JUNIT_REPORT_OUTPUT_DIR)/junit.xml
.PHONY: test-e2e-olm

#
# test-e2e is a future replacement target for test-e2e-olm that is used only upstream CI, until we merge:
# https://github.com/openshift/release/pull/21383
# This PR will make use of CI-managed operator installs/cleanups using the bundle w/o olm_deploy.
#
E2E_RANDOM_SUFFIX:=$(shell echo $$RANDOM)
E2E_TEST_NAMESPACE?="e2e-test-${RANDOM_SUFFIX}"
test-e2e: DEPLOYMENT_NAMESPACE="${E2E_TEST_NAMESPACE}"
test-e2e: $(GO_JUNIT_REPORT) $(JUNITMERGE) $(JUNITREPORT) junitreportdir
	TEST_NAMESPACE=${E2E_TEST_NAMESPACE} DO_SETUP="false" SKIP_CLEANUP="true" hack/test-e2e.sh
	echo "Completed test-e2e"
	$(JUNITMERGE) $$(find $$JUNIT_REPORT_OUTPUT_DIR -iname "*.xml") > $(JUNIT_REPORT_OUTPUT_DIR)/junit.xml
.PHONY: test-e2e

##@ Deployment

deploy: deploy-image ## Deploy operator registry and operator.
	LOCAL_IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY=127.0.0.1:5000/openshift/elasticsearch-operator-registry \
	IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY=127.0.0.1:5000/openshift/elasticsearch-operator-registry \
	$(MAKE) elasticsearch-catalog-build && \
	IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY=image-registry.openshift-image-registry.svc:5000/openshift/elasticsearch-operator-registry \
	IMAGE_ELASTICSEARCH_OPERATOR=image-registry.openshift-image-registry.svc:5000/openshift/origin-elasticsearch-operator:latest \
	$(MAKE) elasticsearch-catalog-deploy && \
	IMAGE_ELASTICSEARCH_OPERATOR=image-registry.openshift-image-registry.svc:5000/openshift/origin-elasticsearch-operator:latest \
	$(MAKE) elasticsearch-operator-install
.PHONY: deploy

deploy-image: image ## Push operator image to cluster registry.
	IMAGE_TAG=$(IMAGE_TAG) hack/deploy-image.sh
.PHONY: deploy-image

deploy-example: # Create an example Elasticsearch custom resource.
	@oc create -n $(DEPLOYMENT_NAMESPACE) -f hack/cr.yaml
.PHONY: deploy-example

scale-cvo:
	@oc -n openshift-cluster-version scale deployment/cluster-version-operator --replicas=$(REPLICAS)
.PHONY: scale-cvo

scale-olm:
	@oc -n openshift-operator-lifecycle-manager scale deployment/olm-operator --replicas=$(REPLICAS)
.PHONY: scale-olm

uninstall:
	$(MAKE) elasticsearch-catalog-uninstall
.PHONY: uninstall

elasticsearch-catalog: elasticsearch-catalog-build elasticsearch-catalog-deploy ## Build and deploy the elasticsearch operator registry.
.PHONY: elasticsearch-catalog

elasticsearch-cleanup: elasticsearch-operator-uninstall elasticsearch-catalog-uninstall ## Cleanup operator-registry and operator deployments.
.PHONY: elasticsearch-cleanup

# builds an operator-registry image containing the elasticsearch operator.
elasticsearch-catalog-build: ## Build elasticsearch operator registry.
	olm_deploy/scripts/catalog-build.sh
.PHONY: elasticsearch-catalog-build

# deploys the operator registry image and creates a catalogsource referencing it
elasticsearch-catalog-deploy: ## Deploy elasticsearch operator registry.
	olm_deploy/scripts/catalog-deploy.sh
.PHONY: elasticsearch-catalog-deploy

# deletes the catalogsource and catalog namespace
elasticsearch-catalog-uninstall: ## Uninstall elasticsearch operator registry.
	olm_deploy/scripts/catalog-uninstall.sh
.PHONY: elasticsearch-catalog-uninstall

# installs the elasticsearch operator from the deployed operator-registry/catalogsource.
elasticsearch-operator-install: ## Install the elasticsearch operator.
	olm_deploy/scripts/operator-install.sh
.PHONY: elasticsearch-operator-install

# uninstalls the elasticsearch operator
elasticsearch-operator-uninstall: ## Uninstall the elasticsearch operator.
	olm_deploy/scripts/operator-uninstall.sh
.PHONY: elasticsearch-operator-uninstall
