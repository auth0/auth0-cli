#-----------------------------------------------------------------------------------------------------------------------
# Variables (https://www.gnu.org/software/make/manual/html_node/Using-Variables.html#Using-Variables)
#-----------------------------------------------------------------------------------------------------------------------
-include .env

.DEFAULT_GOAL := help

NAME := auth0-cli
GO_PKG := github.com/auth0/$(NAME)
GO_BIN ?= $(shell go env GOPATH)/bin
GO_PACKAGES := $(shell go list ./... | grep -vE "vendor|tools|mock")

UNIVERSAL_LOGIN_ASSETS_EXTERNAL_DIR ?= ./../ulp-branding-app
UNIVERSAL_LOGIN_ASSETS_INTERNAL_DIR = ./internal/cli/data/universal-login

## Configuration for build-info
BUILD_DIR ?= $(CURDIR)/out
BUILD_INFO_PKG := $(GO_PKG)/internal/buildinfo
BUILD_USER := $(shell whoami)
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_UNTRACKED_CHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GIT_UNTRACKED_CHANGES),)
	GITCOMMIT := $(GIT_COMMIT)-dirty
endif
GIT_BRANCH ?= $(shell git rev-parse --verify --abbrev-ref HEAD)
GO_LINKER_FLAGS = -X '$(BUILD_INFO_PKG).Version=dev' \
					 -X '$(BUILD_INFO_PKG).Revision=$(GIT_COMMIT)' \
					 -X '$(BUILD_INFO_PKG).Branch=$(GIT_BRANCH)' \
					 -X '$(BUILD_INFO_PKG).BuildUser=$(BUILD_USER)' \
					 -X '$(BUILD_INFO_PKG).BuildDate=$(BUILD_TIME)'

# Colors for the printf
RESET = $(shell tput sgr0)
COLOR_WHITE = $(shell tput setaf 7)
COLOR_BLUE = $(shell tput setaf 4)
COLOR_YELLOW = $(shell tput setaf 3)
TEXT_INVERSE = $(shell tput smso)

#-----------------------------------------------------------------------------------------------------------------------
# Rules (https://www.gnu.org/software/make/manual/html_node/Rule-Introduction.html#Rule-Introduction)
#-----------------------------------------------------------------------------------------------------------------------
.PHONY: help

help: ## Show this help
	@egrep -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

#-----------------------------------------------------------------------------------------------------------------------
# Dependencies
#-----------------------------------------------------------------------------------------------------------------------
.PHONY: deps

deps: ## Download dependencies
	${call print, "Downloading dependencies"}
	@go mod vendor -v

$(GO_BIN)/mockgen:
	${call print, "Installing mockgen"}
	@go install -v github.com/golang/mock/mockgen@latest

$(GO_BIN)/golangci-lint:
	${call print, "Installing golangci-lint"}
	@go install -v github.com/golangci/golangci-lint/cmd/golangci-lint@2059b18a39d559552839476ba78ce6acaa499b43 # v1.59.0

$(GO_BIN)/govulncheck:
	${call print, "Installing go vulnerability checker"}
	@go install golang.org/x/vuln/cmd/govulncheck@latest

$(GO_BIN)/commander:
	${call print, "Installing commander"}
	@go install -v github.com/commander-cli/commander/v2/cmd/commander@latest

$(GO_BIN)/auth0:
	@$(MAKE) install

#-----------------------------------------------------------------------------------------------------------------------
# Assets
#-----------------------------------------------------------------------------------------------------------------------
.PHONY: assets

assets: ## Generate Universal Login embeddable assets
	${call print, "Generating Universal Login embeddable assets"}
	@if [ ! -d "${UNIVERSAL_LOGIN_ASSETS_EXTERNAL_DIR}" ]; \
	then \
	  	${call print_warning, "No such file or directory: ${UNIVERSAL_LOGIN_ASSETS_EXTERNAL_DIR}"}; \
		exit 1; \
	fi
	@rm -rf "${UNIVERSAL_LOGIN_ASSETS_INTERNAL_DIR}"
	@cd "${UNIVERSAL_LOGIN_ASSETS_EXTERNAL_DIR}" && npm install && npm run build
	@cp -r "${UNIVERSAL_LOGIN_ASSETS_EXTERNAL_DIR}/dist" "${UNIVERSAL_LOGIN_ASSETS_INTERNAL_DIR}"

#-----------------------------------------------------------------------------------------------------------------------
# Documentation
#-----------------------------------------------------------------------------------------------------------------------
.PHONY: docs docs-start docs-clean

docs: docs-clean ## Build the documentation
	@go run ./cmd/doc-gen
	@mv ./docs/auth0.md ./docs/index.md

docs-start: ## Start the doc site locally for testing purposes
	@cd docs && bundle install && bundle exec jekyll serve

docs-clean: ## Remove the documentation
	@rm -f ./docs/auth0_*.md

#-----------------------------------------------------------------------------------------------------------------------
# Building & Installing
#-----------------------------------------------------------------------------------------------------------------------
.PHONY: build build-all-platforms install

build: ## Build the cli binary for the native platform
	${call print, "Building the cli binary"}
	go build -v -ldflags "$(GO_LINKER_FLAGS)" -o "${BUILD_DIR}/auth0" cmd/auth0/main.go

build-with-cover: ## Build the cli binary for the native platform with coverage support.
	${call print, "Building the cli binary"}
	go build -cover -v -ldflags "$(GO_LINKER_FLAGS)" -o "${BUILD_DIR}/auth0" cmd/auth0/main.go

build-all-platforms: ## Build a dev version of the cli binary for all supported platforms
	for os in darwin linux windows; \
	do env GOOS=$$os go build -ldflags "$(GO_LINKER_FLAGS)" -o "${BUILD_DIR}/auth0-$${os}" cmd/auth0/main.go; \
	done

install: ## Install the cli binary for the native platform
	${call print, "Installing the cli binary"}
	@$(MAKE) build BUILD_DIR="$(GO_BIN)"

install-with-cover: ## Install the cli binary for the native platform with coverage support.
	${call print, "Installing the cli binary"}
	@$(MAKE) build-with-cover BUILD_DIR="$(GO_BIN)"

#-----------------------------------------------------------------------------------------------------------------------
# Checks
#-----------------------------------------------------------------------------------------------------------------------
.PHONY: lint

lint: $(GO_BIN)/golangci-lint ## Run go linter checks
	${call print, "Running golangci-lint over project"}
	@golangci-lint run -v --fix -c .golangci.yml ./...

check-vuln: $(GO_BIN)/govulncheck ## Check go vulnerabilities
	${call print, "Running govulncheck over project"}
	@govulncheck ./...

check-docs: ## Check that documentation was generated correctly
	${call print, "Checking that documentation was generated correctly"}
	@$(MAKE) docs
	@if [ -n "$$(git status --porcelain)" ]; \
	then \
		echo "Rebuilding the documentation resulted in changed files:"; \
		echo "$$(git diff)"; \
		echo "Please run \`make docs\` to regenerate docs."; \
		exit 1; \
	fi
	@echo "Documentation is generated correctly."

#-----------------------------------------------------------------------------------------------------------------------
# Testing
#-----------------------------------------------------------------------------------------------------------------------
.PHONY: test test-unit test-integration test-mocks

test: test-unit test-integration ## Run all tests

test-unit: ## Run unit tests
	${call print, "Running unit tests"}
	@go test -v -race ${GO_PACKAGES} -coverprofile="coverage-unit-tests.out"

test-integration: install-with-cover $(GO_BIN)/auth0 $(GO_BIN)/commander ## Run integration tests. To run a specific test pass the FILTER var. Usage: `make test-integration FILTER="attack protection"`
	${call print, "Running integration tests"}
	@mkdir -p "coverage"
	@PATH=$(GO_BIN):$$PATH \
		GOCOVERDIR=coverage bash ./test/integration/scripts/run-test-suites.sh
	@go tool covdata textfmt -i "coverage" -o "coverage-integration-tests.out"

test-mocks: $(GO_BIN)/mockgen ## Generate testing mocks using mockgen
	${call print, "Generating test mocks"}
	@go generate -v ./...

test-clean: ## Clean up test tenant
	${call print, "Cleaning up the test tenant"}
	@bash ./test/integration/scripts/test-cleanup.sh

#-----------------------------------------------------------------------------------------------------------------------
# Helpers
#-----------------------------------------------------------------------------------------------------------------------
define print
	@printf "${TEXT_INVERSE}${COLOR_WHITE} :: ${COLOR_BLUE} %-75s ${COLOR_WHITE} ${RESET}\n" $(1)
endef

define print_warning
	printf "${TEXT_INVERSE}${COLOR_WHITE} !! ${COLOR_YELLOW} %-75s ${COLOR_WHITE} ${RESET}\n" $(1)
endef
