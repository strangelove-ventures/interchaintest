default: help

.PHONY: help
help: ## Print this help message
	@echo "Available make commands:"; grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: ibctest
ibctest: ## Build ibctest binary into ./bin
	go test -c -o ./bin/ibctest ./cmd/ibctest

.PHONY: test
test: ## Run unit tests
	@go test -cover -short -race -timeout=60s $(shell go list ./... | grep -v /cmd/)