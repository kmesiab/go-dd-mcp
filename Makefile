.PHONY: all build test lint clean install-tools fmt vet check coverage

# Variables
BINARY_NAME=datadog-mcp-server
GO=go
GOTEST=$(GO) test
GOVET=$(GO) vet
GOFMT=$(GO) fmt
GOLINT=golangci-lint
COVERAGE_FILE=coverage.out

# Default target
all: check build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build -o $(BINARY_NAME) .

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# Run linter
lint:
	@echo "Running golangci-lint..."
	$(GOLINT) run --timeout=5m

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@echo "All tools installed!"

# Run all checks (fmt, vet, lint, test)
check: fmt vet lint test

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f $(COVERAGE_FILE)
	@rm -f coverage.html
	@echo "Clean complete!"

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GO) mod tidy
	$(GO) mod verify

# Run the server (requires env vars)
run:
	@echo "Running $(BINARY_NAME)..."
	@./$(BINARY_NAME)

# Help target
help:
	@echo "Available targets:"
	@echo "  make build          - Build the binary"
	@echo "  make test           - Run tests"
	@echo "  make coverage       - Run tests with coverage report"
	@echo "  make lint           - Run golangci-lint"
	@echo "  make fmt            - Format code"
	@echo "  make vet            - Run go vet"
	@echo "  make check          - Run all checks (fmt, vet, lint, test)"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make tidy           - Tidy and verify dependencies"
	@echo "  make install-tools  - Install development tools"
	@echo "  make run            - Run the server"
	@echo "  make all            - Run checks and build (default)"
