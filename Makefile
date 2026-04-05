.PHONY: all build test test-unit test-integration test-spec coverage lint fmt clean help

# Default target
all: fmt lint build test

# Build the library
build:
	go build -v ./...

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	go test -v -race -coverprofile=coverage.out ./pkg/... ./internal/...

# Run integration tests (requires SampleData.udbx)
test-integration:
	go test -v ./test/integration/...

# Run spec tests (TDD compliance tests)
test-spec:
	go test -v ./test/spec/...

# Generate coverage report
coverage: test-unit
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Lint code
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, running go vet instead"; \
		go vet ./...; \
	fi

# Format code
fmt:
	go fmt ./...

# Clean build artifacts
clean:
	rm -f coverage.out coverage.html
	rm -rf bin/ dist/ build/
	go clean -cache

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run example
example:
	go run ./cmd/udbx4go-example/main.go

# Display help
help:
	@echo "Available targets:"
	@echo "  all              - Format, lint, build, and test"
	@echo "  build            - Build the library"
	@echo "  test             - Run all tests"
	@echo "  test-unit        - Run unit tests with coverage"
	@echo "  test-integration - Run integration tests"
	@echo "  test-spec        - Run spec compliance tests"
	@echo "  coverage         - Generate HTML coverage report"
	@echo "  lint             - Run linter"
	@echo "  fmt              - Format code"
	@echo "  clean            - Clean build artifacts"
	@echo "  deps             - Download and tidy dependencies"
	@echo "  example          - Run example application"
	@echo "  help             - Show this help"
