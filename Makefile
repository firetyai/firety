GO ?= go
BINARY ?= firety
PACKAGE ?= ./cmd/firety
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GOFILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')
LDFLAGS := -s -w \
	-X github.com/firety/firety/internal/platform/buildinfo.version=$(VERSION) \
	-X github.com/firety/firety/internal/platform/buildinfo.commit=$(COMMIT) \
	-X github.com/firety/firety/internal/platform/buildinfo.date=$(DATE)

.PHONY: build fmt fmt-check lint test coverage test-race tidy precommit ci rules-docs package-check

build:
	mkdir -p bin
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PACKAGE)

fmt:
	gofmt -w $(GOFILES)

fmt-check:
	test -z "$$(gofmt -l $(GOFILES))"

lint:
	$(GO) vet ./...

test:
	$(GO) test ./...

coverage:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out

test-race:
	$(GO) test -race ./...

tidy:
	$(GO) mod tidy

package-check:
	npm pack --dry-run >/dev/null

precommit: fmt lint test package-check

ci: fmt-check lint test test-race build package-check

rules-docs:
	mkdir -p docs
	$(GO) run ./cmd/firety-docs > docs/lint-rules.md
