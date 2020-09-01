#!/usr/bin/env make

test:
	CGO_ENABLED=1 go test -race ./... -count 1

lint:
	golangci-lint run -v --timeout=5m

.PHONY: test lint
