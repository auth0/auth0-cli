#!/usr/bin/env make

# setup variables
NAME := auth0-cli
PKG := github.com/auth0/$(NAME)
BUILDINFOPKG := $(PKG)/internal/build-info
GOBIN ?= $(shell go env GOPATH)/bin

## setup variables for build-info
BUILDUSER := $(shell whoami)
BUILDTIME := $(shell date -u '+%Y-%m-%d %H:%M:%S')
VERSION := $(shell git describe --abbrev=0)
GITCOMMIT := $(shell git rev-parse --short HEAD)

GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
	GITCOMMIT := $(GITCOMMIT)-dirty
endif

GITBRANCH ?= $(shell git rev-parse --verify --abbrev-ref HEAD)
CTIMEVAR = -X '$(BUILDINFOPKG).Version=$(VERSION)' \
					 -X '$(BUILDINFOPKG).Revision=$(GITCOMMIT)' \
					 -X '$(BUILDINFOPKG).Branch=$(GITBRANCH)' \
					 -X '$(BUILDINFOPKG).BuildUser=$(BUILDUSER)' \
					 -X '$(BUILDINFOPKG).BuildDate=$(BUILDTIME)'

generate:
	go generate ./...
.PHONY: generate

test:
	CGO_ENABLED=1 go test -race ./... -count 1
.PHONY: test

lint:
	golangci-lint run -v --timeout=5m
.PHONY: lint

# Build for the native platform
build:
	go build -ldflags "$(CTIMEVAR)" -o auth0 cmd/auth0/main.go
.PHONY: build

# Build for the native platform
build:
.PHONY: build

# Build a beta version of auth0-cli for all supported platforms
build-all-platforms:
	env GOOS=darwin go build -ldflags "$(CTIMEVAR)" -o auth0-darwin cmd/auth0/main.go
	env GOOS=linux go build -ldflags "$(CTIMEVAR)" -o auth0-linux cmd/auth0/main.go
	env GOOS=windows go build -ldflags "$(CTIMEVAR)" -o auth0-windows.exe cmd/auth0/main.go
.PHONY: build-all-platforms

# Run all the tests and code checks
ci: build-all-platforms test lint
.PHONY: ci

$(GOBIN)/mockgen:
	@cd && GO111MODULE=on go get github.com/golang/mock/mockgen@v1.4.4

.PHONY: mocks
mocks: $(GOBIN)/mockgen
	go generate ./...

$(GOBIN)/commander:
	cd && GO111MODULE=auto go get github.com/commander-cli/commander/cmd/commander

$(GOBIN)/auth0-cli-config-generator:
	go install ./pkg/auth0-cli-config-generator

run-integration:
	auth0-cli-config-generator && commander test commander.yaml
.PHONY: run-integration

# Delete all test apps created during integration testing
integration-cleanup:
	./integration/test-cleanup.sh
.PHONY: integration-cleanup

integration: $(GOBIN)/auth0-cli-config-generator $(GOBIN)/commander run-integration integration-cleanup
.PHONY: integration
