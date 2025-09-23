#!/bin/bash
set -e

# GANTA Docker Entrypoint Script
# Provides flexible execution modes for the GANTA container

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored messages
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if inventory file exists
check_inventory() {
    if [ ! -f "${INVENTORY_FILE:-/data/inventory.yaml}" ]; then
        log_error "Inventory file not found at ${INVENTORY_FILE:-/data/inventory.yaml}"
        log_info "Please mount your inventory file to /data/inventory.yaml or set INVENTORY_FILE"
        exit 1
    fi
}

# Check if catalog file exists
check_catalog() {
    if [ ! -f "${CATALOG_FILE:-/data/catalog.yaml}" ]; then
        log_error "Catalog file not found at ${CATALOG_FILE:-/data/catalog.yaml}"
        log_info "Please mount your catalog file to /data/catalog.yaml or set CATALOG_FILE"
        exit 1
    fi
}

# Substitute environment variables in files
substitute_env_vars() {
    local file=$1
    local temp_file="/tmp/$(basename $file).tmp"
    
    if [ -f "$file" ]; then
        envsubst < "$file" > "$temp_file"
        mv "$temp_file" "$file"
        log_info "Environment variables substituted in $file"
    fi
}

# Main execution logic
main() {
    # Default file paths
    INVENTORY_FILE=${INVENTORY_FILE:-/data/inventory.yaml}
    CATALOG_FILE=${CATALOG_FILE:-/data/catalog.yaml}
    CONFIG_FILE=${CONFIG_FILE:-/config/config.yaml}
    OUTPUT_DIR=${OUTPUT_DIR:-/data/output}

    # Create output directory if it doesn't exist
    mkdir -p "$OUTPUT_DIR"

    # If no arguments provided, show help
    if [ $# -eq 0 ]; then
        log_info "GANTA Docker Container"
        log_info "====================="
        exec /app/go-anta --help
        exit 0
    fi

    # Handle special commands
    case "$1" in
        "bash"|"sh")
            log_info "Starting shell session..."
            exec /bin/bash
            ;;
        
        "validate")
            log_info "Validating configuration files..."
            check_inventory
            check_catalog
            log_info "Inventory: $INVENTORY_FILE ✓"
            log_info "Catalog: $CATALOG_FILE ✓"
            if [ -f "$CONFIG_FILE" ]; then
                log_info "Config: $CONFIG_FILE ✓"
            fi
            exit 0
            ;;
        
        "example")
            log_info "Copying example files to /data..."
            cp -r /app/examples/* /data/
            log_info "Example files copied successfully"
            exit 0
            ;;
        
        "nrfu")
            log_info "Running NRFU tests..."
            check_inventory
            check_catalog
            
            # Build command with optional parameters
            CMD="/app/go-anta nrfu -i $INVENTORY_FILE -c $CATALOG_FILE"
            
            # Add optional flags
            [ -n "$CONFIG_FILE" ] && [ -f "$CONFIG_FILE" ] && CMD="$CMD -c $CONFIG_FILE"
            [ -n "$OUTPUT_FORMAT" ] && CMD="$CMD -f $OUTPUT_FORMAT"
            [ -n "$OUTPUT_FILE" ] && CMD="$CMD -o $OUTPUT_DIR/$OUTPUT_FILE"
            [ -n "$DEVICE_TAGS" ] && CMD="$CMD -t $DEVICE_TAGS"
            [ -n "$TEST_NAMES" ] && CMD="$CMD -T $TEST_NAMES"
            [ -n "$CONCURRENCY" ] && CMD="$CMD -j $CONCURRENCY"
            [ -n "$HIDE_STATUS" ] && CMD="$CMD --hide $HIDE_STATUS"
            [ "$DRY_RUN" = "true" ] && CMD="$CMD --dry-run"
            [ "$IGNORE_STATUS" = "true" ] && CMD="$CMD --ignore-status"
            
            # Add remaining arguments
            shift
            CMD="$CMD $@"
            
            log_info "Executing: $CMD"
            exec $CMD
            ;;
        
        "check")
            log_info "Running connectivity check..."
            check_inventory
            
            CMD="/app/go-anta check -i $INVENTORY_FILE"
            [ -n "$DEVICE_TAGS" ] && CMD="$CMD -t $DEVICE_TAGS"
            [ -n "$DEVICE_NAMES" ] && CMD="$CMD -d $DEVICE_NAMES"
            
            shift
            CMD="$CMD $@"
            
            log_info "Executing: $CMD"
            exec $CMD
            ;;
        
        "scheduled")
            log_info "Starting scheduled execution mode..."
            SCHEDULE_INTERVAL=${SCHEDULE_INTERVAL:-3600}
            
            while true; do
                log_info "Running scheduled tests at $(date)"
                
                OUTPUT_FILE="results-$(date +%Y%m%d-%H%M%S).json"
                /app/go-anta nrfu \
                    -i "$INVENTORY_FILE" \
                    -c "$CATALOG_FILE" \
                    -f json \
                    -o "$OUTPUT_DIR/$OUTPUT_FILE" || true
                
                log_info "Tests completed. Results saved to $OUTPUT_DIR/$OUTPUT_FILE"
                log_info "Sleeping for $SCHEDULE_INTERVAL seconds..."
                sleep "$SCHEDULE_INTERVAL"
            done
            ;;
        
        "watch")
            log_info "Starting watch mode..."
            WATCH_INTERVAL=${WATCH_INTERVAL:-60}
            
            while true; do
                clear
                echo "GANTA Watch Mode - $(date)"
                echo "================================"
                
                /app/go-anta nrfu \
                    -i "$INVENTORY_FILE" \
                    -c "$CATALOG_FILE" \
                    ${OUTPUT_FORMAT:+-f $OUTPUT_FORMAT} || true
                
                echo ""
                echo "Refreshing in $WATCH_INTERVAL seconds... (Ctrl+C to stop)"
                sleep "$WATCH_INTERVAL"
            done
            ;;
        
        *)
            # Pass through all arguments to go-anta
            exec /app/go-anta "$@"
            ;;
    esac
}

# Run main function with all arguments
main "$@"