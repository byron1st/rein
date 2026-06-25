BINARY_NAME := rein
BUILD_DIR := bin
MAIN_PACKAGE := ./cmd/rein
PKG ?= ./...
TEST ?= .

.PHONY: build test test-single lint lint-fix run tidy clean help

build: ## Build the CLI binary.
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

test: ## Run all tests with race detection and coverage.
	go test ./...

test-single: ## Run one test by name, e.g. make test-single PKG=./pkg/agent TEST=TestLoop.
	go test -run "$(TEST)" $(PKG)

lint: ## Check formatting and run go vet.
	@files=$$(gofmt -l .); if [ -n "$$files" ]; then echo "$$files"; exit 1; fi
	go vet ./...

lint-fix: ## Format Go files.
	go fmt ./...

run: ## Run the CLI entrypoint.
	go run $(MAIN_PACKAGE)

tidy: ## Clean up module files.
	go mod tidy

clean: ## Remove generated build and coverage artifacts.
	rm -rf $(BUILD_DIR) coverage.out coverage.html

help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*## "; printf "Available targets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
