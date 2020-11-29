# Auto generated binary variables helper managed by https://github.com/bwplotka/bingo v0.2.3. DO NOT EDIT.
# All tools are designed to be build inside $GOBIN.
GOPATH ?= $(shell go env GOPATH)
GOBIN  ?= $(firstword $(subst :, ,${GOPATH}))/bin
GO     ?= $(shell which go)

# Bellow generated variables ensure that every time a tool under each variable is invoked, the correct version
# will be used; reinstalling only if needed.
# For example for bingo variable:
#
# In your main Makefile (for non array binaries):
#
#include .bingo/Variables.mk # Assuming -dir was set to .bingo .
#
#command: $(BINGO)
#	@echo "Running bingo"
#	@$(BINGO) <flags/args..>
#
BINGO := $(GOBIN)/bingo-v0.2.3
$(BINGO): .bingo/bingo.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/bingo-v0.2.3"
	@cd .bingo && $(GO) build -mod=mod -modfile=bingo.mod -o=$(GOBIN)/bingo-v0.2.3 "github.com/bwplotka/bingo"

GO_JUNIT_REPORT := $(GOBIN)/go-junit-report-v0.9.1
$(GO_JUNIT_REPORT): .bingo/go-junit-report.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/go-junit-report-v0.9.1"
	@cd .bingo && $(GO) build -mod=mod -modfile=go-junit-report.mod -o=$(GOBIN)/go-junit-report-v0.9.1 "github.com/jstemmer/go-junit-report"
CONTROLLER_GEN := $(GOBIN)/controller-gen-v0.3.0
$(CONTROLLER_GEN): .bingo/controller-gen.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/controller-gen-v0.3.0"
	@cd .bingo && $(GO) build -mod=mod -modfile=controller-gen.mod -o=$(GOBIN)/controller-gen-v0.3.0 "sigs.k8s.io/controller-tools/cmd/controller-gen"

GOFUMPORTS := $(GOBIN)/gofumports-v0.0.0-20201027171050-85d5401eb0f6
$(GOFUMPORTS): .bingo/gofumports.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/gofumports-v0.0.0-20201027171050-85d5401eb0f6"
	@cd .bingo && $(GO) build -mod=mod -modfile=gofumports.mod -o=$(GOBIN)/gofumports-v0.0.0-20201027171050-85d5401eb0f6 "mvdan.cc/gofumpt/gofumports"

GOLANGCI_LINT := $(GOBIN)/golangci-lint-v1.27.0
$(GOLANGCI_LINT): .bingo/golangci-lint.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/golangci-lint-v1.27.0"
	@cd .bingo && $(GO) build -mod=mod -modfile=golangci-lint.mod -o=$(GOBIN)/golangci-lint-v1.27.0 "github.com/golangci/golangci-lint/cmd/golangci-lint"

JUNITMERGE := $(GOBIN)/junitmerge-v0.0.0-20201103150245-a5287ef1495b
$(JUNITMERGE): .bingo/junitmerge.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/junitmerge-v0.0.0-20201103150245-a5287ef1495b"
	@cd .bingo && $(GO) build -mod=mod -modfile=junitmerge.mod -o=$(GOBIN)/junitmerge-v0.0.0-20201103150245-a5287ef1495b "github.com/openshift/release/tools/junitmerge"

JUNITREPORT := $(GOBIN)/junitreport-v0.0.0-20201103082000-d8009dcf7503
$(JUNITREPORT): .bingo/junitreport.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/junitreport-v0.0.0-20201103082000-d8009dcf7503"
	@cd .bingo && $(GO) build -mod=mod -modfile=junitreport.mod -o=$(GOBIN)/junitreport-v0.0.0-20201103082000-d8009dcf7503 "github.com/openshift/release/tools/junitreport"

KUSTOMIZE := $(GOBIN)/kustomize-v3.5.4
$(KUSTOMIZE): .bingo/kustomize.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/kustomize-v3.5.4"
	@cd .bingo && $(GO) build -mod=mod -modfile=kustomize.mod -o=$(GOBIN)/kustomize-v3.5.4 "sigs.k8s.io/kustomize/kustomize/v3"

OPERATOR_SDK := $(GOBIN)/operator-sdk-v1.2.0
$(OPERATOR_SDK): .bingo/operator-sdk.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/operator-sdk-v1.2.0"
	@cd .bingo && $(GO) build -mod=mod -modfile=operator-sdk.mod -o=$(GOBIN)/operator-sdk-v1.2.0 "github.com/operator-framework/operator-sdk/cmd/operator-sdk"

OPM := $(GOBIN)/opm-v1.13.6
$(OPM): .bingo/opm.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/opm-v1.13.6"
	@cd .bingo && $(GO) build -mod=mod -modfile=opm.mod -o=$(GOBIN)/opm-v1.13.6 "github.com/operator-framework/operator-registry/cmd/opm"

PROMTOOL := $(GOBIN)/promtool-v1.8.2-0.20200522113006-f4dd45609a05
$(PROMTOOL): .bingo/promtool.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/promtool-v1.8.2-0.20200522113006-f4dd45609a05"
	@cd .bingo && $(GO) build -mod=mod -modfile=promtool.mod -o=$(GOBIN)/promtool-v1.8.2-0.20200522113006-f4dd45609a05 "github.com/prometheus/prometheus/cmd/promtool"

