# Trove development tasks. Run `make help` for the list.
SHELL := /bin/sh

GO ?= go
BINARY ?= bin/trove

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X 'Trove/internal/app.Version=$(VERSION)' \
	-X 'Trove/internal/app.Commit=$(COMMIT)' \
	-X 'Trove/internal/app.BuildDate=$(BUILD_DATE)'

# Pinned developer tools, installed into .tools/bin (git-ignored) so every
# machine and CI runs the same versions.
TOOLBIN := $(CURDIR)/.tools/bin
GOIMPORTS_VERSION := v0.52.0
GOLANGCI_LINT_VERSION := v2.15.0
GOIMPORTS := $(TOOLBIN)/goimports
GOLANGCI_LINT := $(TOOLBIN)/golangci-lint

.DEFAULT_GOAL := help
.PHONY: help run fmt fmt-check lint test build check docs-check bench tools clean

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-14s %s\n", $$1, $$2}'

run: ## Run locally with a throwaway data directory
	@data_dir=$$(mktemp -d); echo "data dir: $$data_dir"; $(GO) run ./cmd/server -data-dir "$$data_dir"

fmt: $(GOIMPORTS) ## Format code
	gofmt -w .
	$(GOIMPORTS) -w .

fmt-check: $(GOIMPORTS) ## Fail if code is not formatted
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
	@test -z "$$($(GOIMPORTS) -l .)" || { echo "goimports needed:"; $(GOIMPORTS) -l .; exit 1; }

lint: $(GOLANGCI_LINT) ## Run go vet and golangci-lint
	$(GO) vet ./...
	$(GOLANGCI_LINT) run

test: ## Run unit, integration and HTTP tests with the race detector
	$(GO) test -race -count=1 ./...

build: ## Build the self-contained binary with version info
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/server

check: fmt-check lint test build docs-check ## Run everything CI runs

docs-check: ## Check Markdown links and the docs index
	./scripts/check-docs.sh

bench: ## Run the 100,000-item performance check (milestone M9)
	$(GO) test -run '^$$' -bench . -benchmem ./...

tools: $(GOIMPORTS) $(GOLANGCI_LINT) ## Install pinned developer tools into .tools/bin

$(GOIMPORTS):
	@mkdir -p $(TOOLBIN)
	GOBIN=$(TOOLBIN) $(GO) install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

$(GOLANGCI_LINT):
	@mkdir -p $(TOOLBIN)
	GOBIN=$(TOOLBIN) $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

clean: ## Remove build output and developer tools
	rm -rf bin .tools
