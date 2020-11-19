CURPATH=$(PWD)

export GOBIN=$(CURDIR)/bin
export PATH:=$(GOBIN):$(PATH)

include .bingo/Variables.mk

export GOROOT=$(shell go env GOROOT)
export GOFLAGS=-mod=vendor
export GO111MODULE=on

export OCP_VERSION=4.7

export APP_NAME=elasticsearch-operator

export ARTIFACT_DIR?=./tmp/artifacts
export JUNIT_REPORT_OUTPUT_DIR=$(ARTIFACT_DIR)/junit
COVERAGE_DIR=$(ARTIFACT_DIR)/coverage

IMAGE_TAG?=127.0.0.1:5000/openshift/origin-$(APP_NAME):latest
APP_REPO=github.com/openshift/$(APP_NAME)
KUBECONFIG?=$(HOME)/.kube/config
MAIN_PKG=cmd/manager/main.go
RUN_LOG?=elasticsearch-operator.log
RUN_PID?=elasticsearch-operator.pid
LOGGING_IMAGE_STREAM?=stable
OPERATOR_NAMESPACE=openshift-operators-redhat
DEPLOYMENT_NAMESPACE?=openshift-logging
REPLICAS?=0
OS_NAME=$(shell uname -s | tr '[:upper:]' '[:lower:]')

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
generate: $(GEN_TIMESTAMP) $(OPERATOR_SDK)
$(GEN_TIMESTAMP): $(shell find pkg/apis -name '*.go')
	@./hack/generate-crd.sh
	@$(MAKE) fmt
	@touch $@

regenerate: $(OPERATOR_SDK)
	@rm -f $(GEN_TIMESTAMP)
	@$(MAKE) generate

build:
	@go build -o $(GOBIN)/elasticsearch-operator $(MAIN_PKG)

clean:
	@rm -rf bin tmp _output
	go clean -cache -testcache ./...

fmt: $(GOFUMPORTS)
	@$(GOFUMPORTS) -l -w $(shell find pkg cmd test -name '*.go')

lint: $(GOLANGCI_LINT) fmt lint-prom
	@$(GOLANGCI_LINT) run -c golangci.yaml

lint-prom: $(PROMTOOL)
	@$(PROMTOOL) check rules ./files/prometheus_rules.yml
	@$(PROMTOOL) check rules ./files/prometheus_alerts.yml

image:
	@if [ $${SKIP_BUILD:-false} = false ] ; then \
		podman build -t $(IMAGE_TAG) . ; \
	fi

test-unit: $(GO_JUNIT_REPORT) coveragedir junitreportdir test-unit-prom
	@go test -v -race -coverprofile=$(COVERAGE_DIR)/test-unit.cov ./pkg/... ./cmd/... 2>&1 | tee /dev/stderr | $(GO_JUNIT_REPORT) > $(JUNIT_REPORT_OUTPUT_DIR)/junit.xml
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

deploy-example: deploy deploy-example-secret
	@oc create -n $(DEPLOYMENT_NAMESPACE) -f hack/cr.yaml
.PHONY: deploy-example

deploy-example-secret: gen-example-certs
	hack/deploy-example-secrets.sh $(DEPLOYMENT_NAMESPACE)
.PHONY: deploy-example-secret

gen-example-certs:
	@rm -rf /tmp/example-secrets ||: \
	mkdir /tmp/example-secrets && \
	hack/cert_generation.sh /tmp/example-secrets $(DEPLOYMENT_NAMESPACE) elasticsearch
.PHONY: gen-example-certs

run: deploy deploy-example
	@ALERTS_FILE_PATH=files/prometheus_alerts.yml \
	RULES_FILE_PATH=files/prometheus_rules.yml \
	OPERATOR_NAME=elasticsearch-operator WATCH_NAMESPACE=$(DEPLOYMENT_NAMESPACE) \
	KUBERNETES_CONFIG=/etc/origin/master/admin.kubeconfig \
	go run ${MAIN_PKG} > $(RUN_LOG) 2>&1 & echo $$! > $(RUN_PID)

run-local:
	@ALERTS_FILE_PATH=files/prometheus_alerts.yml \
	RULES_FILE_PATH=files/prometheus_rules.yml \
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

MANIFEST_VERSION?="4.7"
generate-bundle: regenerate $(OPM)
	mkdir -p bundle; \
	$(OPM) alpha bundle generate --directory manifests/${MANIFEST_VERSION} --package elasticsearch-operator --channels ${MANIFEST_VERSION} --default ${MANIFEST_VERSION} --output-dir bundle/; \
	find bundle/manifests/ -type f ! \( -name "elasticsearch-operator*.yaml" -o -name "*crd.yaml" \) -delete && \
	$(OPERATOR_SDK) bundle validate --verbose bundle
.PHONY: generate-bundle

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
# or the image format:
# IMAGE_FORMAT
#
# You must also set:
# ELASTICSEARCH_OPERATOR_NAMESPACE (Default: openshift-operators-redhat)
RANDOM_SUFFIX:=$(shell echo $$RANDOM)
TEST_NAMESPACE?="e2e-test-${RANDOM_SUFFIX}"
test-e2e-olm: DEPLOYMENT_NAMESPACE="${TEST_NAMESPACE}"
test-e2e-olm: $(GO_JUNIT_REPORT) $(JUNITMERGE) $(JUNITREPORT) junitreportdir
	TEST_NAMESPACE=${TEST_NAMESPACE} hack/test-e2e-olm.sh
	$(JUNITMERGE) $$(find $$JUNIT_REPORT_OUTPUT_DIR -iname *.xml) > $(JUNIT_REPORT_OUTPUT_DIR)/junit.xml
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
