# iknow-ingestion Makefile
# Install golangci-lint: https://golangci-lint.run/usage/install/
# Copy .env.example to .env and fill in values before running locally.

BINARY    := webhook
IMAGE     ?= ghcr.io/screener-site/iknow-ingestion
TAG       ?= latest

.DEFAULT_GOAL := help

## help: print this help message
.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	  /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

## build: compile all packages
.PHONY: build
build: ## compile all packages
	go build ./...

## test: run all tests
.PHONY: test
test: ## run all tests
	go test ./...

## run-webhook: run the webhook server (reads config from env / .env)
.PHONY: run-webhook
run-webhook: ## run the webhook HTTP server
	go run ./cmd/webhook

## lint: run golangci-lint (install: https://golangci-lint.run/usage/install/)
.PHONY: lint
lint: ## run golangci-lint
	golangci-lint run ./...

## docker-build: build the webhook Docker image
.PHONY: docker-build
docker-build: ## build the webhook container image
	docker build -t $(IMAGE):$(TAG) .

## docker-push: push the webhook Docker image to the registry
.PHONY: docker-push
docker-push: ## push the webhook container image to the registry
	docker push $(IMAGE):$(TAG)

## tidy: run go mod tidy
.PHONY: tidy
tidy: ## tidy go module dependencies
	go mod tidy
