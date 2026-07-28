.PHONY: setup test cover fmt ci build clean lint help examples

setup: ## Install optional local tooling hints
	@echo "Install golangci-lint from https://golangci-lint.run/ if you want 'make lint'"
	@echo "cover tooling is provided by the Go distribution (go tool cover)"

test: ## Run all tests with race detector and coverage profile
	echo 'mode: atomic' > coverage.txt && go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.txt -race -timeout=30s ./...

cover: test ## Run tests and open the coverage report
	go tool cover -html=coverage.txt

fmt: ## Format all Go sources with gofmt
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

lint: ## Run golangci-lint (requires local install)
	golangci-lint run ./...

ci: test lint ## Run tests and lint checks

build: ## Build all packages
	go build -v ./...

examples: ## Build example programs
	go build -o /tmp/haproxy-spoe-examples/ ./examples/...

clean: ## Remove temporary files
	go clean
	rm -f coverage.txt

# Absolutely awesome: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := build
