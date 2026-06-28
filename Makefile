BINARY_NAME := rein
BUILD_DIR := bin
MAIN_PACKAGE := ./cmd/rein
PKG ?= ./...
TEST ?= .

.PHONY: mockgen build test test-single test-mutation test-mutation-pkg lint lint-fix run tidy clean help

mockgen: ## Generate mocks for interfaces in the project
	command -v mockery >/dev/null 2>&1 || go install github.com/vektra/mockery/v3@latest
	rm -rf pkg/mocks && mkdir pkg/mocks
	mockery
	find pkg/mocks -type f -name '*.go' -exec perl -pi -e 's/interface\{\}/any/g' {} +
	find pkg/mocks -type f -name '*.go' -exec gofmt -w {} +

build: ## Build the CLI binary.
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

clean-testcache: ## Clean test cache to ensure tests run with the latest code changes
	@go clean -testcache

test: clean-testcache ## Clean test caches and run tests
	@go test ./...

race: clean-testcache ## Clean test caches and run race tests
	@go test -short -race ./...

test-single: ## Run one test by name, e.g. make test-single PKG=./pkg/agent TEST=TestLoop.
	@go test -run "$(TEST)" $(PKG)

test-mutation: ## Run mutation testing and show survived mutants
	@command -v gremlins >/dev/null 2>&1 || go install github.com/go-gremlins/gremlins/cmd/gremlins@latest
	@gremlins unleash -S l

test-mutation-pkg: ## Run mutation testing for a single package, for example: make test-mutation-pkg PKG=./internal/service/keysharing
	@test -n "$(PKG)" && test "$(PKG)" != "./..." || (echo "usage: make test-mutation-pkg PKG=./pkg/llm" >&2; exit 1)
	@command -v gremlins >/dev/null 2>&1 || go install github.com/go-gremlins/gremlins/cmd/gremlins@latest
	@gremlins unleash "$(PKG)" -S l --workers=3 --timeout-coefficient=20

check: format lint ## Run linters and modernize checks

format: ## Format and modernize code
	@go fmt ./...
	@go fix ./...

lint: ## Run linters to check code quality and style
	@go mod tidy
	@golangci-lint run

run: ## Run the CLI entrypoint.
	go run $(MAIN_PACKAGE)

tidy: ## Clean up module files.
	go mod tidy

clean: ## Remove generated build and coverage artifacts.
	rm -rf $(BUILD_DIR) coverage.out coverage.html

help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*## "; printf "Available targets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
