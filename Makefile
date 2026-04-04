.PHONY: help format lint test security ci build clean

GO ?= go

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the golem binary
	$(GO) build ./cmd/golem

format: ## Format code
	$(GO) fmt ./...

lint: ## Lint (style/static analysis)
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run

test: ## Run tests
	$(GO) test ./...

security: ## Security + dependency checks
	$(GO) vet ./...
	$(GO) mod verify
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

ci: format lint test security ## Run everything (CI parity)

clean: ## Remove build artifacts
	rm -rf build/ golem
