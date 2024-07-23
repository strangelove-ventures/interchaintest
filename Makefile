DOCKER := $(shell which docker)
protoVer=0.13.2
protoImageName=ghcr.io/cosmos/proto-builder:$(protoVer)
protoImage=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace $(protoImageName)

default: help

.PHONY: help
help: ## Print this help message
	@echo "Available make commands:"; grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: interchaintest
interchaintest: gen ## Build interchaintest binary into ./bin
	go test -ldflags "-X github.com/ODINPROTOCOL/interchaintest/v8/interchaintest.GitSha=$(shell git describe --always --dirty)" -c -o ./bin/interchaintest ./cmd/interchaintest

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