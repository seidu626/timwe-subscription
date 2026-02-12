#!/bin/bash

# Resubscribe Processor Runner Script
# This script provides convenient ways to run the resubscribe processor

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Default values
CONFIG_FILE="config.json"
BASE_URL="http://localhost:8083"
TELCO="AirtelTigo"
ENTRY_CHANNEL="USSD"
PRODUCT_IDS="8509"
WAIT_TIME="30s"
DEBUG=false
DRY_RUN=false
METRICS_PORT=":9102"

# Function to print usage
print_usage() {
    echo -e "${BLUE}Resubscribe Processor Runner${NC}"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -c, --config FILE     Configuration file (default: config.json)"
    echo "  -u, --url URL         Base URL of subscription service (default: http://localhost:8083)"
    echo "  -t, --telco TELCO     Telco name (default: AirtelTigo)"
    echo "  -e, --channel CHANNEL Entry channel (default: USSD)"
    echo "  -p, --products IDS    Comma-separated product IDs (default: 8509)"
    echo "  -w, --wait TIME       Wait time between calls (default: 30s)"
    echo "  -d, --debug           Enable debug logging"
    echo "  --dry-run             Dry run mode (show what would be done)"
    echo "  --metrics-port PORT   Metrics port (default: :9102)"
    echo "  -h, --help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Run with default config"
    echo "  $0 -c custom_config.json             # Run with custom config"
    echo "  $0 -t Vodafone -p 8509,14392        # Run with overrides"
    echo "  $0 --dry-run                         # Show what would be done"
    echo ""
}

# Function to check if binary exists
check_binary() {
    if [[ ! -f "./resubscribe-processor" ]]; then
        echo -e "${YELLOW}Binary not found. Building...${NC}"
        if command -v make &> /dev/null; then
            make build
        else
            echo -e "${RED}Make not found. Please build manually:${NC}"
            echo "go build -o resubscribe-processor ."
            exit 1
        fi
    fi
}

# Function to create logs directory
create_logs_dir() {
    if [[ ! -d "./logs" ]]; then
        mkdir -p ./logs
        echo -e "${GREEN}Created logs directory${NC}"
    fi
}

# Function to run the processor
run_processor() {
    local args=()
    
    # Add config file if specified
    if [[ -n "$CONFIG_FILE" ]]; then
        args+=("-config" "$CONFIG_FILE")
    fi
    
    # Add command line overrides
    if [[ -n "$BASE_URL" ]]; then
        args+=("-url" "$BASE_URL")
    fi
    
    if [[ -n "$TELCO" ]]; then
        args+=("-telco" "$TELCO")
    fi
    
    if [[ -n "$ENTRY_CHANNEL" ]]; then
        args+=("-channel" "$ENTRY_CHANNEL")
    fi
    
    if [[ -n "$PRODUCT_IDS" ]]; then
        args+=("-products" "$PRODUCT_IDS")
    fi
    
    if [[ -n "$WAIT_TIME" ]]; then
        args+=("-wait" "$WAIT_TIME")
    fi
    
    if [[ "$DEBUG" == true ]]; then
        args+=("-debug")
    fi
    
    if [[ "$DRY_RUN" == true ]]; then
        args+=("-dry-run")
    fi
    
    if [[ -n "$METRICS_PORT" ]]; then
        args+=("-metrics-addr" "$METRICS_PORT")
    fi
    
    echo -e "${GREEN}Starting Resubscribe Processor...${NC}"
    echo -e "${BLUE}Command: ./resubscribe-processor ${args[*]}${NC}"
    echo ""
    
    # Run the processor
    ./resubscribe-processor "${args[@]}"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--config)
            CONFIG_FILE="$2"
            shift 2
            ;;
        -u|--url)
            BASE_URL="$2"
            shift 2
            ;;
        -t|--telco)
            TELCO="$2"
            shift 2
            ;;
        -e|--channel)
            ENTRY_CHANNEL="$2"
            shift 2
            ;;
        -p|--products)
            PRODUCT_IDS="$2"
            shift 2
            ;;
        -w|--wait)
            WAIT_TIME="$2"
            shift 2
            ;;
        -d|--debug)
            DEBUG=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --metrics-port)
            METRICS_PORT="$2"
            shift 2
            ;;
        -h|--help)
            print_usage
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            print_usage
            exit 1
            ;;
    esac
done

# Main execution
main() {
    echo -e "${BLUE}=== Resubscribe Processor Runner ===${NC}"
    echo ""
    
    # Check if binary exists
    check_binary
    
    # Create logs directory
    create_logs_dir
    
    # Validate config file if specified
    if [[ -n "$CONFIG_FILE" && ! -f "$CONFIG_FILE" ]]; then
        echo -e "${RED}Configuration file not found: $CONFIG_FILE${NC}"
        exit 1
    fi
    
    # Show configuration
    echo -e "${YELLOW}Configuration:${NC}"
    echo "  Config File:    $CONFIG_FILE"
    echo "  Base URL:       $BASE_URL"
    echo "  Telco:          $TELCO"
    echo "  Entry Channel:  $ENTRY_CHANNEL"
    echo "  Product IDs:    $PRODUCT_IDS"
    echo "  Wait Time:      $WAIT_TIME"
    echo "  Debug:          $DEBUG"
    echo "  Dry Run:        $DRY_RUN"
    echo "  Metrics Port:   $METRICS_PORT"
    echo ""
    
    # Run the processor
    run_processor
}

# Run main function
main "$@"