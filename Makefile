SHELL := /bin/sh

.DEFAULT_GOAL := help

ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

CONTAINER_RUNTIME ?= docker
GO_VERSION ?= 1.27.1
GOLANGCI_LINT_VERSION ?= 2.13.2
MOCKERY_VERSION ?= 3.7
SQLC_VERSION ?= 1.31.1

BUF_IMAGE ?= bufbuild/buf
GO_IMAGE ?= golang:$(GO_VERSION)-alpine
GOLANGCI_LINT_IMAGE ?= golangci/golangci-lint:v$(GOLANGCI_LINT_VERSION)-alpine
MOCKERY_IMAGE ?= vektra/mockery:$(MOCKERY_VERSION)
SQLC_IMAGE ?= sqlc/sqlc:$(SQLC_VERSION)

run: ## Run the application
	$(CONTAINER_RUNTIME) compose -p hive up

generate-sql: ## Generate sqlc code
	$(CONTAINER_RUNTIME) run --rm \
		-v "$(ROOT_DIR):/src" \
		-w "/src" \
		"$(SQLC_IMAGE)" generate; \

proto-lint: ## Lint protobuf definitions
	$(CONTAINER_RUNTIME) run --rm \
		-v "$(ROOT_DIR):/defs" \
		-w /defs \
		"$(BUF_IMAGE)" lint

proto-generate: proto-clean ## Generate protobuf code
	$(CONTAINER_RUNTIME) run --rm \
		-v "$(ROOT_DIR):/defs" \
		-w /defs \
		"$(BUF_IMAGE)" generate

proto-clean: ## Remove generated protobuf code
	rm -rf "$(ROOT_DIR)/pkg/grpc"
