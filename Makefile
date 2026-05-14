SHELL := /bin/bash

BINARY  := patchrun
PKG     := ./cmd/patchrun
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)
BIN_DIR := bin
GO      ?= go

.PHONY: all build test lint vet fmt fmt-check cover race install clean help

all: lint test build

build: ## Build the patchrun binary into ./bin/patchrun
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) $(PKG)

install: ## go install patchrun to $GOBIN
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' $(PKG)

test: ## Run all tests with -race
	$(GO) test -race -count=1 ./...

race: test

vet: ## go vet
	$(GO) vet ./...

fmt: ## gofmt -s -w on all files
	gofmt -s -w .

fmt-check: ## gofmt -l (fail if any files need formatting)
	@out=$$(gofmt -s -l .); \
	if [ -n "$$out" ]; then \
		echo "Files need gofmt:"; echo "$$out"; exit 1; \
	fi

lint: fmt-check vet ## Run formatters and static checks

cover: ## Generate coverage report (cross-package via -coverpkg)
	$(GO) test -race -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1
	@echo "HTML report: $(GO) tool cover -html=coverage.out"

clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
