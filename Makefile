CURPATH=$(PWD)

export GOBIN=$(CURDIR)/bin
export PATH:=$(GOBIN):$(PATH)

include .bingo/Variables.mk

export GOROOT=$(shell go env GOROOT)
export GOFLAGS=-mod=vendor
export GO111MODULE=on

export LOGGING_VERSION=5.2

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

GO_FILES       := $(shell find . -type f -name '*.go')
MANIFEST_FILES := $(shell find manifests/ -type f)
OTHER_FILES    := $(shell find files/ -type f)

.PHONY: all build clean fmt generate gobindir run test-e2e test-unit

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
generate: $(GEN_TIMESTAMP) $(OPERATOR_SDK) $(CONTROLLER_GEN)
$(GEN_TIMESTAMP): $(shell find apis -name '*.go')
	@$(CONTROLLER_GEN) object paths="./apis/..."
	@$(CONTROLLER_GEN) crd:crdVersions=v1 rbac:roleName=elasticsearch-operator paths="./..." output:crd:artifacts:config=config/crd/bases
	@$(MAKE) fmt
	@touch $@

regenerate: $(OPERATOR_SDK) $(CONTROLLER_GEN)
	@rm -f $(GEN_TIMESTAMP)
	@$(MAKE) generate

build:
	@go build -o $(GOBIN)/elasticsearch-operator $(MAIN_PKG)

clean:
	@rm -rf bin tmp _output
	go clean -cache -testcache ./...

fmt: $(GOFUMPORTS)
	@$(GOFUMPORTS) -l -w $(shell find internal apis controllers test version -name '*.go') ./*.go

lint: $(GOLANGCI_LINT) fmt lint-prom lint-dockerfile
	@GOLANGCI_LINT_CACHE="$(CURDIR)/.cache" $(GOLANGCI_LINT) run -c golangci.yaml

lint-prom: $(PROMTOOL)
	@$(PROMTOOL) check rules ./files/prometheus_recording_rules.yml
	@$(PROMTOOL) check rules ./files/prometheus_alerts.yml

lint-dockerfile:
	@hack/lint-dockerfile
.PHONY: lint-dockerfile

image: .output/image
.output/image: gen-dockerfiles $(GO_FILES) $(MANIFEST_FILES) $(OTHER_FILES)
	podman build -f Dockerfile.dev -t $(IMAGE_TAG) .
	@touch $@

test-unit: $(GO_JUNIT_REPORT) coveragedir junitreportdir test-unit-prom
	@set -o pipefail && \
		go test -race -coverprofile=$(COVERAGE_DIR)/test-unit.cov ./internal/... ./apis/... ./controllers/... ./. 2>&1 | \
		tee /dev/stderr | \
		$(GO_JUNIT_REPORT) > $(JUNIT_REPORT_OUTPUT_DIR)/junit.xml
	@grep -v 'zz_generated\.' $(COVERAGE_DIR)/test-unit.cov > $(COVERAGE_DIR)/nogen.cov
	@go tool cover -html=$(COVERAGE_DIR)/nogen.cov -o $(COVERAGE_DIR)/test-unit-coverage.html
	@go tool cover -func=$(COVERAGE_DIR)/nogen.cov | tail -n 1

test-unit-prom: $(PROMTOOL)
	@$(PROMTOOL) test rules ./test/files/prometheus-unit-tests/test.yml


deploy: deploy-image
	LOCAL_IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY=127.0.0.1:5000/openshift/elasticsearch-operator-registry \
	$(MAKE) elasticsearch-catalog-build && \
	IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY=image-registry.openshift-image-registry.svc:5000/openshift/elasticsearch-operator-registry \
	IMAGE_ELASTICSEARCH_OPERATOR=image-registry.openshift-image-registry.svc:5000/openshift/origin-elasticsearch-operator:latest \
	$(MAKE) elasticsearch-catalog-deploy && \
	IMAGE_ELASTICSEARCH_OPERATOR=image-registry.openshift-image-registry.svc:5000/openshift/origin-elasticsearch-operator:latest \
	$(MAKE) elasticsearch-operator-install

.PHONY: deploy

deploy-image: image
	IMAGE_TAG=$(IMAGE_TAG) hack/deploy-image.sh
.PHONY: deploy-image

deploy-example:
	@oc create -n $(DEPLOYMENT_NAMESPACE) -f hack/cr.yaml
.PHONY: deploy-example

run: deploy deploy-example
	@ALERTS_FILE_PATH=files/prometheus_alerts.yml \
	RULES_FILE_PATH=files/prometheus_recording_rules.yml \
	OPERATOR_NAME=elasticsearch-operator WATCH_NAMESPACE=$(DEPLOYMENT_NAMESPACE) \
	KUBERNETES_CONFIG=/etc/origin/master/admin.kubeconfig \
	go run ${MAIN_PKG} > $(RUN_LOG) 2>&1 & echo $$! > $(RUN_PID)

run-local:
	@ALERTS_FILE_PATH=files/prometheus_alerts.yml \
	RULES_FILE_PATH=files/prometheus_recording_rules.yml \
	OPERATOR_NAME=elasticsearch-operator WATCH_NAMESPACE=$(DEPLOYMENT_NAMESPACE) \
	KUBERNETES_CONFIG=$(KUBECONFIG) \
	go run ${MAIN_PKG} LOG_LEVEL=debug
.PHONY: run-local

scale-cvo:
	@oc -n openshift-cluster-version scale deployment/cluster-version-operator --replicas=$(REPLICAS)
.PHONY: scale-cvo

scale-olm:
	@oc -n openshift-operator-lifecycle-manager scale deployment/olm-operator --replicas=$(REPLICAS)
.PHONY: scale-olm

uninstall:
	$(MAKE) elasticsearch-catalog-uninstall
.PHONY: uninstall

# Generate bundle manifests and metadata, then validate generated files.
# - the bundle manifests are copied to ./manifests/${LOGGING_VERSION}/, e.g., ./manifests/4.7/
BUNDLE_VERSION?=$(LOGGING_VERSION).0
# Options for 'bundle-build'
BUNDLE_CHANNELS := --channels=stable,stable-${LOGGING_VERSION}
BUNDLE_DEFAULT_CHANNEL := --default-channel=stable
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

bundle: regenerate $(KUSTOMIZE)
	$(OPERATOR_SDK) generate kustomize manifests -q
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(BUNDLE_VERSION) $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK) bundle validate ./bundle
	cp bundle/manifests/elasticsearch-operator.clusterserviceversion.yaml  manifests/${LOGGING_VERSION}/elasticsearch-operator.v${BUNDLE_VERSION}.clusterserviceversion.yaml
	cp bundle/manifests/logging.openshift.io_elasticsearches.yaml  manifests/${LOGGING_VERSION}/logging.openshift.io_elasticsearches_crd.yaml
	cp bundle/manifests/logging.openshift.io_kibanas.yaml  manifests/${LOGGING_VERSION}/logging.openshift.io_kibanas_crd.yaml
	cp bundle/manifests/elasticsearch-operator-metrics-monitor_monitoring.coreos.com_v1_servicemonitor.yaml  manifests/${LOGGING_VERSION}/
	cp bundle/manifests/elasticsearch-operator-metrics_v1_service.yaml  manifests/${LOGGING_VERSION}/
	cp bundle/manifests/leader-election-role_rbac.authorization.k8s.io_v1_role.yaml manifests/${LOGGING_VERSION}/
	cp bundle/manifests/leader-election-rolebinding_rbac.authorization.k8s.io_v1_rolebinding.yaml manifests/${LOGGING_VERSION}/
	cp bundle/manifests/metrics-reader_rbac.authorization.k8s.io_v1beta1_clusterrole.yaml manifests/${LOGGING_VERSION}/
	cp bundle/manifests/proxy-role_rbac.authorization.k8s.io_v1_clusterrole.yaml manifests/${LOGGING_VERSION}/
	cp bundle/manifests/proxy-rolebinding_rbac.authorization.k8s.io_v1_clusterrolebinding.yaml manifests/${LOGGING_VERSION}/
	cp bundle/manifests/prometheus_rbac.authorization.k8s.io_v1_role.yaml manifests/${LOGGING_VERSION}/
	cp bundle/manifests/prometheus_rbac.authorization.k8s.io_v1_rolebinding.yaml manifests/${LOGGING_VERSION}/
.PHONY: bundle

test-e2e-upgrade: 
	hack/testing-olm-upgrade/test-030-olm-upgrade-n-1-n.sh
.PHONY: test-e2e-upgrade

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
test-e2e-olm: DEPLOYMENT_NAMESPACE="${TEST_NAMESPACE}"
test-e2e-olm: $(GO_JUNIT_REPORT) $(JUNITMERGE) $(JUNITREPORT) junitreportdir
	TEST_NAMESPACE=${TEST_NAMESPACE} hack/test-e2e-olm.sh
	echo "Complete e2e olm test"
	$(JUNITMERGE) $$(find $$JUNIT_REPORT_OUTPUT_DIR -iname "*.xml") > $(JUNIT_REPORT_OUTPUT_DIR)/junit.xml
.PHONY: test-e2e-olm

elasticsearch-catalog: elasticsearch-catalog-build elasticsearch-catalog-deploy

elasticsearch-cleanup: elasticsearch-operator-uninstall elasticsearch-catalog-uninstall

# builds an operator-registry image containing the elasticsearch operator
elasticsearch-catalog-build:
	olm_deploy/scripts/catalog-build.sh

# deploys the operator registry image and creates a catalogsource referencing it
elasticsearch-catalog-deploy:
	olm_deploy/scripts/catalog-deploy.sh

# deletes the catalogsource and catalog namespace
elasticsearch-catalog-uninstall:
	olm_deploy/scripts/catalog-uninstall.sh

# installs the elasticsearch operator from the deployed operator-registry/catalogsource.
elasticsearch-operator-install:
	olm_deploy/scripts/operator-install.sh

# uninstalls the elasticsearch operator
elasticsearch-operator-uninstall:
	olm_deploy/scripts/operator-uninstall.sh
gen-dockerfiles:
	./hack/generate-dockerfile-from-midstream > Dockerfile && \
	./hack/generate-dockerfile-from-midstream Dockerfile.in dev-meta.yaml > Dockerfile.dev
.PHONY: gen-dockerfiles
