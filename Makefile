SHELL:=/bin/bash
LINT_FILE_TAG=master
LINT_FILE_URL=https://raw.githubusercontent.com/tuihub/librarian/$(LINT_FILE_TAG)/.golangci.yml
LINT_FILE_LOCAL=.golangci.yml

.PHONY: init
# init env
init:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: lint
# lint files
lint:
	curl -sSL $(LINT_FILE_URL) -o $(LINT_FILE_LOCAL)
	golangci-lint run --fix -c $(LINT_FILE_LOCAL)
	golangci-lint run -c $(LINT_FILE_LOCAL) # re-run to make sure fixes are valid, useful in some condition

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
