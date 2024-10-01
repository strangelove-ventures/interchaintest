DOCKER := $(shell which docker)
protoVer=0.13.2
protoImageName=ghcr.io/cosmos/proto-builder:$(protoVer)
protoImage=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace $(protoImageName)
golangci_lint_cmd=golangci-lint
golangci_version=v1.61.0
gofumpt_cmd=gofumpt
gofumpt_version=v0.7.0

default: help

.PHONY: help
help: ## Print this help message
	@echo "Available make commands:"; grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: interchaintest
interchaintest: gen ## Build interchaintest binary into ./bin
	go test -ldflags "-X github.com/strangelove-ventures/interchaintest/v8/interchaintest.GitSha=$(shell git describe --always --dirty)" -c -o ./bin/interchaintest ./cmd/interchaintest

.PHONY: test
test: ## Run unit tests
	@go test -cover -short -race -timeout=60s ./...

.PHONY: docker-reset
docker-reset: ## Attempt to delete all running containers. Useful if interchaintest does not exit cleanly.
	@docker stop $(shell docker ps -q) &>/dev/null || true
	@docker rm --force $(shell docker ps -q) &>/dev/null || true

.PHONY: docker-mac-nuke
docker-mac-nuke: ## macOS only. Try docker-reset first. Kills and restarts Docker Desktop.
	killall -9 Docker && open /Applications/Docker.app

.PHONY: gen
gen: ## Run code generators
	go generate ./...

.PHONY: mod-tidy
mod-tidy: ## Run mod tidy
	go mod tidy
	cd local-interchain && go mod tidy

.PHONY: proto-gen
proto-gen: ## Generate code from protos
	@echo "Generating Protobuf files"
	@$(protoImage) sh ./scripts/protocgen.sh

.PHONY: lint
lint: ## Lint the repository
	@echo "--> Running linter"
	@if ! $(golangci_lint_cmd) --version 2>/dev/null | grep -q $(golangci_version); then \
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(golangci_version); \
  fi
	      @$(golangci_lint_cmd) run ./... --timeout 15m

.PHONY: lint-fix
lint-fix: ## Lint the repository and fix warnings (if applicable)
	@echo "--> Running linter and fixing issues"
	@if ! $(golangci_lint_cmd) --version 2>/dev/null | grep -q $(golangci_version); then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(golangci_version); \
	fi
	@$(golangci_lint_cmd) run ./... --fix --timeout 15m

.PHONY: gofumpt
gofumpt: ## Format the code with gofumpt
	@echo "--> Running gofumpt"
	@if ! $(gofumpt_cmd) -version 2>/dev/null | grep -q $(gofumpt_version); then \
		go install mvdan.cc/gofumpt@$(gofumpt_version); \
	fi
	@gofumpt -l -w .