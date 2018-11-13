CURPATH=$(PWD)
TARGET_DIR=$(CURPATH)/_output

GOBUILD=go build
GOPATH=$(TARGET_DIR):$(TARGET_DIR)/vendor:$(CURPATH)/cmd

IMAGE_BUILD_OPTS=
IMAGE_BUILDER?=imagebuilder

APP_NAME=elasticsearch-operator
APP_REPO=github.com/openshift/$(APP_NAME)
TARGET=$(TARGET_DIR)/bin/$(APP_NAME)
IMAGE_TAG=quay.io/openshift/$(APP_NAME)
MAIN_PKG=cmd/$(APP_NAME)/main.go

# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

#.PHONY: all build clean install uninstall fmt simplify check run
.PHONY: all build clean fmt simplify run

all: build #check install

build: $(SRC)
	@mkdir -p $(TARGET_DIR)/src/$(APP_REPO)
	@cp -ru $(CURPATH)/pkg $(TARGET_DIR)/src/$(APP_REPO)
	@cp -ru $(CURPATH)/vendor/* $(TARGET_DIR)/src
	@GOPATH=$(GOPATH) $(GOBUILD) $(LDFLAGS) -o $(TARGET) $(MAIN_PKG)

clean:
	@rm -rf $(TARGET_DIR)

image:
	$(IMAGE_BUILDER) -t $(IMAGE_TAG) . $(IMAGE_BUILD_OPTS)

fmt:
	@gofmt -l -w $(SRC)

simplify:
	@gofmt -s -l -w $(SRC)

deploy: deploy-setup image deploy-image
	hack/deploy.sh
.PHONY: deploy

deploy-image:
	hack/deploy-image.sh
.PHONY: deploy-image

deploy-example:
	@oc create -n openshift-logging -f hack/cr.yaml
.PHONY: deploy-example

deploy-setup:
	EXCLUSIONS="05-deployment.yaml image-references" hack/deploy-setup.sh
.PHONY: deploy-setup

undeploy:
	hack/undeploy.sh
.PHONY: undeploy
