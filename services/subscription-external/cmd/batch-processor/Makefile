# Makefile for Batch Processor

# Variables
BINARY_NAME=batch-processor
BUILD_DIR=.
GO=/usr/local/go/bin/go
GOFLAGS=-v

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run the batch processor with default config
run: build
	./$(BINARY_NAME) -config config.json

# Run in dry-run mode to see what would be executed
dry-run: build
	./$(BINARY_NAME) -config config.json -dry-run

# Run once for testing (only process start count)
run-once: build
	./$(BINARY_NAME) -config config.json -once

# Run with debug logging
debug: build
	./$(BINARY_NAME) -config config.json -debug

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f batch_processor.log
	@rm -f batch_results_*.json
	@echo "Clean complete"

# Install as systemd service (requires sudo)
install-service:
	@echo "Installing systemd service..."
	@sudo cp batch-processor.service /etc/systemd/system/
	@sudo systemctl daemon-reload
	@sudo systemctl enable batch-processor.service
	@echo "Service installed. Use 'sudo systemctl start batch-processor' to start"

# Uninstall systemd service (requires sudo)
uninstall-service:
	@echo "Uninstalling systemd service..."
	@sudo systemctl stop batch-processor || true
	@sudo systemctl disable batch-processor || true
	@sudo rm -f /etc/systemd/system/batch-processor.service
	@sudo systemctl daemon-reload
	@echo "Service uninstalled"

# Setup cron job
setup-cron:
	@echo "Setting up cron job..."
	@chmod +x run_batch_processor.sh
	@echo "Add the following to your crontab (crontab -e):"
	@echo "0 2 * * * $(PWD)/run_batch_processor.sh"

# View logs
logs:
	@tail -f batch_processor.log

# Test with small batch
test:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	./$(BINARY_NAME) -start 1000 -max 3000 -increment 1000 -wait 5s -debug

# Show help
help:
	@echo "Available targets:"
	@echo "  make build          - Build the batch processor"
	@echo "  make run            - Run with default config"
	@echo "  make dry-run        - Show what would be executed without actually running"
	@echo "  make run-once       - Run only once for the start count"
	@echo "  make debug          - Run with debug logging"
	@echo "  make test           - Test with small batch (1000-3000)"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make install-service - Install as systemd service (requires sudo)"
	@echo "  make uninstall-service - Uninstall systemd service (requires sudo)"
	@echo "  make setup-cron     - Setup cron job"
	@echo "  make logs           - Tail the log file"
	@echo "  make help           - Show this help message"

.PHONY: build run dry-run run-once debug clean install-service uninstall-service setup-cron logs test help
