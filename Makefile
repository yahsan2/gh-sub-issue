.PHONY: build install test clean help

# Variables
BINARY_NAME=gh-sub-issue
GO_FILES=$(shell find . -name '*.go' -type f)

# Default target
all: build

## help: Show this help message
help:
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## build: Build the binary
build:
	go build -o $(BINARY_NAME) .

## install: Install as gh extension
install: build
	gh extension remove sub-issue 2>/dev/null || true
	gh extension install .

## test: Run tests
test:
	go test -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## clean: Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

## fmt: Format code
fmt:
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run linter (requires golangci-lint)
lint:
	golangci-lint run

## dev: Build and install for development
dev: fmt vet build install
	@echo "✓ Development build installed"