# Makefile for remap Go CLI tool

# Project information
BINARY_NAME=remap
MODULE_NAME=remap
MAIN_PACKAGE=.

# Build information
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go commands are used directly

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.commit=$(COMMIT)"
BUILD_FLAGS=-trimpath $(LDFLAGS)

# Coverage
COVERAGE_DIR=./coverage
COVERAGE_PROFILE=$(COVERAGE_DIR)/coverage.out
COVERAGE_HTML=$(COVERAGE_DIR)/coverage.html

# Tools are installed to default Go bin directory

# Colors for output
NO_COLOR=\033[0m
OK_COLOR=\033[32;01m
ERROR_COLOR=\033[31;01m
WARN_COLOR=\033[33;01m

# Default target
.DEFAULT_GOAL := build

# Phony targets
.PHONY: help build test cover clean tidy audit tools install run fmt deps upgrade outdated

## help: Show this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: Build the binary
build: fmt
	@printf "$(OK_COLOR)==> Building $(BINARY_NAME)...$(NO_COLOR)\n"
	go build $(BUILD_FLAGS) -o $(BINARY_NAME) $(MAIN_PACKAGE)
	@printf "$(OK_COLOR)==> Binary built: $(BINARY_NAME)$(NO_COLOR)\n"

## test: Run all tests
test:
	@printf "$(OK_COLOR)==> Running tests...$(NO_COLOR)\n"
	go test -race -v ./...

## cover: Run tests with coverage
cover:
	@printf "$(OK_COLOR)==> Running tests with coverage...$(NO_COLOR)\n"
	@mkdir -p $(COVERAGE_DIR)
	go test -race -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic ./...
	@go tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	@printf "$(OK_COLOR)==> Coverage report generated: $(COVERAGE_HTML)$(NO_COLOR)\n"
	@go tool cover -func=$(COVERAGE_PROFILE) | grep total | awk '{print "Total coverage: " $$3}'

## clean: Clean build artifacts and coverage files
clean:
	@printf "$(OK_COLOR)==> Cleaning...$(NO_COLOR)\n"
	go clean
	@rm -rf $(COVERAGE_DIR)
	@rm -f ./$(BINARY_NAME)

## tidy: Format code and tidy modules
tidy: fmt
	@printf "$(OK_COLOR)==> Tidying modules...$(NO_COLOR)\n"
	go mod tidy

## fmt: Format Go code
fmt:
	@printf "$(OK_COLOR)==> Formatting code...$(NO_COLOR)\n"
	gofmt -s -w $(shell find . -name '*.go' -not -path './arbo_test/*')

## audit: Run all code quality and security checks
audit:
	@printf "$(OK_COLOR)==> Running go vet...$(NO_COLOR)\n"
	go vet ./...
	@printf "$(OK_COLOR)==> Running golangci-lint...$(NO_COLOR)\n"
	golangci-lint run ./...
	@printf "$(OK_COLOR)==> Running revive...$(NO_COLOR)\n"
	revive -config .revive.toml -formatter friendly ./...
	@printf "$(OK_COLOR)==> Running staticcheck...$(NO_COLOR)\n"
	staticcheck ./...
	@printf "$(OK_COLOR)==> Checking for vulnerabilities...$(NO_COLOR)\n"
	govulncheck ./...
	@printf "$(OK_COLOR)==> All audits completed$(NO_COLOR)\n"

## tools: Install development tools
tools:
	@printf "$(OK_COLOR)==> Installing development tools...$(NO_COLOR)\n"
	go install github.com/mgechev/revive@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@printf "$(OK_COLOR)==> Tools installed to Go bin directory$(NO_COLOR)\n"

## install: Install the binary to GOPATH/bin
install: build
	@printf "$(OK_COLOR)==> Installing $(BINARY_NAME)...$(NO_COLOR)\n"
	go install $(LDFLAGS) $(MAIN_PACKAGE)

## deps: Download dependencies
deps:
	@printf "$(OK_COLOR)==> Downloading dependencies...$(NO_COLOR)\n"
	go mod download

## upgrade: Upgrade all dependencies
upgrade:
	@printf "$(OK_COLOR)==> Upgrading dependencies...$(NO_COLOR)\n"
	go get -u ./...
	go mod tidy

## outdated: Check for outdated dependencies
outdated:
	@printf "$(OK_COLOR)==> Checking for outdated dependencies...$(NO_COLOR)\n"
	go list -u -m -json all | jq -r 'select(.Update) | "\(.Path): \(.Version) -> \(.Update.Version)"'

## version: Show version information
version:
	@echo "Version: $(VERSION)"
	@echo "Build time: $(BUILD_TIME)"
	@echo "Commit: $(COMMIT)"

## info: Show project information
info:
	@echo "Binary name: $(BINARY_NAME)"
	@echo "Module name: $(MODULE_NAME)"
	@echo "Go version: $(shell go version)"

# Development targets
.PHONY: dev watch

## watch: Watch for changes and run tests
watch:
	@printf "$(OK_COLOR)==> Watching for changes...$(NO_COLOR)\n"
	@which fswatch > /dev/null || (echo "$(ERROR_COLOR)fswatch not found. Install with 'brew install fswatch'$(NO_COLOR)" && exit 1)
	fswatch -o . | xargs -n1 -I{} make test

# Quality gates
.PHONY: ci pre-commit

## ci: Run all CI checks (used in CI/CD)
ci: deps tidy audit test cover
	@printf "$(OK_COLOR)==> All CI checks passed$(NO_COLOR)\n"

## pre-commit: Run pre-commit checks
pre-commit: tidy test
	@printf "$(OK_COLOR)==> Pre-commit checks completed$(NO_COLOR)\n"