.PHONY: gen lint format clean help

# Buf commands
gen: ## Generate code from proto files
	buf generate

lint: ## Run linting
	buf lint

format: ## Format proto files
	buf format -w

clean: ## Clean generated code
	rm -rf ../internal/gen/go
	rm -rf ../services/gen/python

# Development
watch: ## Watch for changes and regenerate
	buf generate --watch

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'