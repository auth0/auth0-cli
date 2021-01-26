#!/usr/bin/env make

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
	go build -o auth0 cmd/auth0/main.go
.PHONY: build

# Build for the native platform
build:
.PHONY: build

# Build a beta version of stripe for all supported platforms
build-all-platforms:
	env GOOS=darwin go build -o auth0-darwin cmd/auth0/main.go
	env GOOS=linux go build -o auth0-linux cmd/auth0/main.go
	env GOOS=windows go build -o auth0-windows.exe cmd/auth0/main.go
.PHONY: build-all-platforms

# Run all the tests and code checks
ci: build-all-platforms test lint
.PHONY: ci
