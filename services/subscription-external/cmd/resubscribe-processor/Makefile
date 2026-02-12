# Makefile for Resubscribe Processor

.PHONY: build clean test run help

# Binary name
BINARY_NAME=resubscribe-processor

# Build directory
BUILD_DIR=./build

# Go build flags
LDFLAGS=-ldflags "-X main.Version=$(shell git describe --tags --always --dirty) -X main.BuildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S')"

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for production (optimized)
build-prod:
	@echo "Building $(BINARY_NAME) for production..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Production build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -rf ./logs
	@echo "Clean complete"

# Run the processor
run: build
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

# Run with custom config
run-config: build
	@echo "Running $(BINARY_NAME) with custom config..."
	@$(BUILD_DIR)/$(BINARY_NAME) -config config.json

# Run in debug mode
run-debug: build
	@echo "Running $(BINARY_NAME) in debug mode..."
	@$(BUILD_DIR)/$(BINARY_NAME) -debug

# Run once (single execution)
run-once: build
	@echo "Running $(BINARY_NAME) once..."
	@$(BUILD_DIR)/$(BINARY_NAME) -once

# Dry run (show what would be done)
dry-run: build
	@echo "Running $(BINARY_NAME) in dry-run mode..."
	@$(BUILD_DIR)/$(BINARY_NAME) -dry-run

# Test the code
test:
	@echo "Running tests..."
	go test -v ./...

# Test with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Create logs directory
logs:
	@mkdir -p ./logs

# Install the binary to system path
install: build
	@echo "Installing $(BINARY_NAME)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

# Uninstall the binary
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstallation complete"

# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  build-prod   - Build optimized binary for production"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Build and run the processor"
	@echo "  run-config   - Run with config.json"
	@echo "  run-debug    - Run in debug mode"
	@echo "  run-once     - Run once (single execution)"
	@echo "  dry-run      - Show what would be done without executing"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  deps         - Install dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code"
	@echo "  logs         - Create logs directory"
	@echo "  install      - Install binary to system path"
	@echo "  uninstall    - Remove binary from system path"
	@echo "  help         - Show this help message"