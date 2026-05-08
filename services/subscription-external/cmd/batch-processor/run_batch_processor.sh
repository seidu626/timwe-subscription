#!/bin/bash

# Batch Processor Cron Script
# This script can be run via cron to process batches periodically

# Set working directory
WORK_DIR="/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external"
LOG_DIR="$WORK_DIR/logs"
BATCH_PROCESSOR="$WORK_DIR/cmd/batch-processor/batch-processor"
CONFIG_FILE="$WORK_DIR/cmd/batch-processor/config.json"

# Create log directory if it doesn't exist
mkdir -p "$LOG_DIR"

# Log file with timestamp
LOG_FILE="$LOG_DIR/batch_processor_$(date +%Y%m%d_%H%M%S).log"

# Check if batch processor is already running
if pgrep -f "batch-processor" > /dev/null; then
    echo "$(date): Batch processor is already running. Skipping." >> "$LOG_FILE"
    exit 0
fi

# Build the batch processor if not exists
if [ ! -f "$BATCH_PROCESSOR" ]; then
    echo "$(date): Building batch processor..." >> "$LOG_FILE"
    cd "$WORK_DIR/cmd/batch-processor"
    /usr/local/go/bin/go build -o batch-processor main.go
    if [ $? -ne 0 ]; then
        echo "$(date): Failed to build batch processor" >> "$LOG_FILE"
        exit 1
    fi
fi

# Run the batch processor
echo "$(date): Starting batch processor..." >> "$LOG_FILE"
cd "$WORK_DIR"

# Run with specific parameters
# You can modify these parameters as needed
"$BATCH_PROCESSOR" \
    -config "$CONFIG_FILE" \
    -start 1000 \
    -max 5000000 \
    -increment 1000 \
    -telco "AirtelTigo" \
    -channel "USSD" \
    -products "8509" \
    -wait 30s \
    >> "$LOG_FILE" 2>&1 &

PROCESSOR_PID=$!
echo "$(date): Batch processor started with PID $PROCESSOR_PID" >> "$LOG_FILE"

# Optional: Wait for completion (remove the & above if you want synchronous execution)
# wait $PROCESSOR_PID
# echo "$(date): Batch processor completed" >> "$LOG_FILE"
