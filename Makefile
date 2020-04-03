CURPATH=$(PWD)

export GOROOT=$(shell go env GOROOT)
export GOFLAGS=-mod=vendor
export GO111MODULE=on
export GOBIN=$(CURDIR)/bin
export PATH:=$(CURDIR)/bin:$(PATH)

IMAGE_BUILDER_OPTS=
IMAGE_BUILDER?=imagebuilder
IMAGE_BUILD=$(IMAGE_BUILDER)
export IMAGE_TAGGER?=docker tag

export APP_NAME=elasticsearch-operator
IMAGE_TAG?=quay.io/openshift/origin-$(APP_NAME):latest
export IMAGE_TAG
APP_REPO=github.com/openshift/$(APP_NAME)
KUBECONFIG?=$(HOME)/.kube/config
MAIN_PKG=cmd/manager/main.go
RUN_LOG?=elasticsearch-operator.log
RUN_PID?=elasticsearch-operator.pid
LOGGING_IMAGE_STREAM?=stable
OPERATOR_NAMESPACE=openshift-operators-redhat
DEPLOYMENT_NAMESPACE=openshift-logging
REPLICAS?=0
OS_NAME=$(shell uname -s | tr '[:upper:]' '[:lower:]')

.PHONY: all build clean fmt generate gobindir gosec imagebuilder operator-sdk run sec test-e2e test-unit

all: build

gobindir:
	@mkdir -p $(GOBIN)

GOSEC_VERSION?=2.2.0
GOSEC_URL=https://github.com/securego/gosec/releases/download/v${GOSEC_VERSION}/gosec_${GOSEC_VERSION}_${OS_NAME}_amd64.tar.gz
gosec: gobindir
	@type -p gosec > /dev/null && \
	gosec version | grep -q $(GOSEC_VERSION) || \
	curl -sSfL  ${GOSEC_URL} | tar -z -C ./bin -x $@
	@chmod +x $(GOBIN)/$@

imagebuilder: gobindir
	@if [ $${USE_IMAGE_STREAM:-false} = false ] && ! type -p imagebuilder > /dev/null ; \
	then GOFLAGS="" GO111MODULE=off go get -u github.com/openshift/imagebuilder/cmd/imagebuilder ; \
	fi

OPERATOR_SDK_VERSION?=v0.16.0
OPERATOR_SDK_URL=https://github.com/operator-framework/operator-sdk/releases/download/${OPERATOR_SDK_VERSION}/operator-sdk-${OPERATOR_SDK_VERSION}-$(shell uname -i)-${OS_NAME}-gnu
operator-sdk: gobindir
	@type -p operator_sdk > /dev/null && \
	operator-sdk version | grep -q $(OPERATOR_SDK_VERSION) || \
	curl -sSfL -o $(GOBIN)/$@ ${OPERATOR_SDK_URL}
	@chmod +x $(GOBIN)/$@

GOLANGCI_LINT_VERSION?=1.24.0
GOLANGCI_LINT_URL=https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VERSION}/golangci-lint-${GOLANGCI_LINT_VERSION}-${OS_NAME}-amd64.tar.gz
golangci-lint: gobindir
	@type -p golangci-lint > /dev/null && \
	golangci-lint version | grep -q $(GOLANGCI_LINT_VERSION) || \
	curl -sSfL ${GOLANGCI_LINT_URL} | tar -z --strip-components=1 -C ./bin -x golangci-lint-${GOLANGCI_LINT_VERSION}-${OS_NAME}-amd64/$@
	@chmod +x $(GOBIN)/$@\

GEN_TIMESTAMP=.zz_generate_timestamp
generate: $(GEN_TIMESTAMP)
$(GEN_TIMESTAMP): $(shell find pkg/apis -name '*.go')
	@$(MAKE) operator-sdk
	operator-sdk generate k8s
	operator-sdk generate crds
	@$(MAKE) fmt
	@touch $@

regenerate:
	@rm -f $(GEN_TIMESTAMP)
	@$(MAKE) generate

build: fmt
	@go build -o $(GOBIN)/elasticsearch-operator $(MAIN_PKG)

clean:
	@rm bin/*
	go clean -cache -testcache ./...

fmt:
	@gofmt -s -l -w $(shell find pkg cmd test -name '*.go')

lint: golangci-lint fmt
	@golangci-lint run -c golangci.yaml

image: imagebuilder
	@if [ $${USE_IMAGE_STREAM:-false} = false ] && [ $${SKIP_BUILD:-false} = false ] ; \
	then hack/build-image.sh $(IMAGE_TAG) $(IMAGE_BUILDER) $(IMAGE_BUILDER_OPTS) ; \
	fi

test-e2e: gen-example-certs
	LOGGING_IMAGE_STREAM=$(LOGGING_IMAGE_STREAM) REMOTE_CLUSTER=true hack/test-e2e.sh

test-unit:
	@go test -v ./pkg/... ./cmd/...

test-sec: gosec
	@gosec -severity medium -confidence medium -exclude G304 -quiet ./...

deploy: deploy-image
	hack/deploy.sh
.PHONY: deploy

deploy-no-build:
	hack/deploy.sh
.PHONY: deploy-no-build

deploy-image: image
	hack/deploy-image.sh
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

undeploy:
	hack/undeploy.sh
.PHONY: undeploy


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
test-e2e-olm: gen-example-certs
	hack/test-e2e-olm.sh

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
