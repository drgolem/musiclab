.PHONY: all build test test-verbose test-race test-coverage vet lint fmt clean help

# Default target
all: build test

# Build the main binary
build:
	@echo "Building musiclab..."
	@mkdir -p bin
	go build -o bin/musiclab

# Build all packages
build-all:
	@echo "Building all packages..."
	go build ./...

# Run unit tests
test:
	@echo "Running tests..."
	go test ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	go test -v ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	go test -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Run golangci-lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./...

# Run gofumpt (stricter than gofmt)
fmt:
	@echo "Formatting code..."
	gofumpt -w .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Show help
help:
	@echo "Available targets:"
	@echo "  make build          - Build main binary to bin/musiclab"
	@echo "  make build-all      - Build all packages"
	@echo "  make test           - Run unit tests"
	@echo "  make test-verbose   - Run tests with verbose output"
	@echo "  make test-race      - Run tests with race detector"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make vet            - Run go vet"
	@echo "  make lint           - Run golangci-lint"
	@echo "  make fmt            - Format code with gofumpt"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make all            - Build and test (default)"
	@echo "  make help           - Show this help message"
