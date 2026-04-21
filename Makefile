BINARY_NAME := jira-cli
MODULE      := github.com/fatecannotbealtered/jira-cli
CMD_PATH    := ./cmd/jira-cli
BIN_DIR     := bin

# Version from git tag, fallback to dev
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     := -s -w -X $(MODULE)/cmd.version=$(VERSION)

.PHONY: build test vet lint fmt clean install snapshot help

## build: compile the binary into bin/
build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)

## test: run all unit tests with race detection
test:
	go test -race ./...

## vet: run static analysis
vet:
	go vet ./...

## lint: run golangci-lint (install if missing)
lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

## fmt: check formatting (fails if unformatted)
fmt:
	@test -z "$$(gofmt -l .)" || (echo "Run gofmt -w . to fix formatting" && gofmt -l . && exit 1)

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR) dist

## install: build and install to GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" $(CMD_PATH)

## snapshot: build a local goreleaser snapshot (no publish)
snapshot:
	goreleaser release --snapshot --clean

## help: show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
