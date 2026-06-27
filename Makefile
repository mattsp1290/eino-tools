GOLANGCI_LINT_VERSION := v2.12.2
GOLANGCI_LINT_TOOLCHAIN := go1.26.0

.PHONY: lint test vet

# golangci-lint must be built with a Go version compatible with .golangci.yml.
lint:
	GOTOOLCHAIN=$(GOLANGCI_LINT_TOOLCHAIN) go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

test:
	go test ./...

vet:
	go vet ./...
