#!/bin/bash
# Deploy Renewal System Script
# This script deploys the opt-out/opt-in renewal system

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
LOG_DIR="/var/log/subscription"
CONFIG_DIR="/etc/subscription"
SERVICE_USER="subscription"
SERVICE_GROUP="subscription"

echo -e "${GREEN}=== Deploying Opt-Out/Opt-In Renewal System ===${NC}"

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
if [[ $EUID -eq 0 ]]; then
    print_error "This script should not be run as root"
    exit 1
fi

# Check if required directories exist
if [[ ! -d "$PROJECT_ROOT" ]]; then
    print_error "Project root directory not found: $PROJECT_ROOT"
    exit 1
fi

# Load environment variables
if [[ -f "$CONFIG_DIR/.env" ]]; then
    print_status "Loading environment variables from $CONFIG_DIR/.env"
    source "$CONFIG_DIR/.env"
else
    print_warning "Environment file not found at $CONFIG_DIR/.env"
    print_warning "Please ensure environment variables are set"
fi

# 1. Create necessary directories
print_status "Creating necessary directories..."
sudo mkdir -p "$LOG_DIR"
sudo mkdir -p "$CONFIG_DIR"
sudo mkdir -p "/opt/subscription"

# 2. Set ownership
print_status "Setting directory ownership..."
sudo chown -R "$SERVICE_USER:$SERVICE_GROUP" "$LOG_DIR"
sudo chown -R "$SERVICE_USER:$SERVICE_GROUP" "$CONFIG_DIR"
sudo chown -R "$SERVICE_USER:$SERVICE_GROUP" "/opt/subscription"

# 3. Backup current data
print_status "Creating database backup..."
BACKUP_FILE="backup_$(date +%Y%m%d_%H%M%S).sql"
if command -v pg_dump &> /dev/null; then
    if [[ -n "$DB_USER" && -n "$DB_NAME" ]]; then
        pg_dump -U "$DB_USER" -d "$DB_NAME" > "$BACKUP_FILE"
        print_status "Database backup created: $BACKUP_FILE"
    else
        print_warning "Database credentials not found, skipping backup"
    fi
else
    print_warning "pg_dump not found, skipping database backup"
fi

# 4. Run database migrations
print_status "Running database migrations..."
if [[ -f "$PROJECT_ROOT/migrations/003_renewal_optout_optin.sql" ]]; then
    if [[ -n "$DB_USER" && -n "$DB_NAME" ]]; then
        psql -U "$DB_USER" -d "$DB_NAME" -f "$PROJECT_ROOT/migrations/003_renewal_optout_optin.sql"
        print_status "Database migration completed"
    else
        print_error "Database credentials not found, cannot run migration"
        exit 1
    fi
else
    print_error "Migration file not found: 003_renewal_optout_optin.sql"
    exit 1
fi

# 5. Copy configuration files
print_status "Installing configuration files..."
sudo cp "$PROJECT_ROOT/config/renewal.yaml" "$CONFIG_DIR/"
sudo chown "$SERVICE_USER:$SERVICE_GROUP" "$CONFIG_DIR/renewal.yaml"

# 6. Build the renewal worker
print_status "Building renewal worker..."
cd "$PROJECT_ROOT"
if command -v go &> /dev/null; then
    go build -o "/opt/subscription/renewal-worker" ./cmd/renewal-worker
    print_status "Renewal worker built successfully"
else
    print_warning "Go compiler not found, skipping build"
    print_warning "Please build the renewal worker manually"
fi

# 7. Create systemd service
print_status "Creating systemd service..."
sudo tee /etc/systemd/system/renewal-worker.service > /dev/null << EOF
[Unit]
Description=Subscription Renewal Worker (Opt-Out/Opt-In)
After=network.target postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_GROUP
WorkingDirectory=/opt/subscription
ExecStart=/opt/subscription/renewal-worker
Restart=always
RestartSec=10
StandardOutput=append:$LOG_DIR/renewal-worker.log
StandardError=append:$LOG_DIR/renewal-worker-error.log
Environment=CONFIG_FILE=$CONFIG_DIR/renewal.yaml
Environment=LOG_LEVEL=info

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=$LOG_DIR /opt/subscription

[Install]
WantedBy=multi-user.target
EOF

# 8. Create cron jobs for monitoring
print_status "Setting up monitoring cron jobs..."
sudo tee /etc/cron.d/renewal-monitor > /dev/null << EOF
# Monitor renewal worker health
*/5 * * * * $SERVICE_USER /opt/subscription/check-renewal-health.sh

# Daily churn evaluation
0 1 * * * $SERVICE_USER /opt/subscription/evaluate-churns.sh

# Retry failed opt-ins
*/30 * * * * $SERVICE_USER /opt/subscription/retry-failed-optins.sh

# Clean up old logs
0 2 * * * $SERVICE_USER /opt/subscription/cleanup-logs.sh
EOF

# 9. Create monitoring scripts
print_status "Creating monitoring scripts..."

# Health check script
sudo tee /opt/subscription/check-renewal-health.sh > /dev/null << 'EOF'
#!/bin/bash
# Check renewal worker health

LOG_FILE="/var/log/subscription/health-check.log"
ALERT_WEBHOOK="${ALERT_WEBHOOK_URL:-}"

log_message() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOG_FILE"
}

# Check if worker is running
if ! systemctl is-active --quiet renewal-worker; then
    log_message "ALERT: Renewal worker is not running!"
    systemctl restart renewal-worker
    
    # Send alert if webhook configured
    if [[ -n "$ALERT_WEBHOOK" ]]; then
        curl -X POST "$ALERT_WEBHOOK" -d '{
            "text": "Renewal worker was down and has been restarted",
            "level": "warning"
        }' 2>/dev/null || true
    fi
fi

# Check for stuck renewals
if command -v psql &> /dev/null && [[ -n "$DB_USER" && -n "$DB_NAME" ]]; then
    STUCK_COUNT=$(psql -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT COUNT(*) FROM renewal_cycles 
        WHERE billing_status = 'PENDING' 
        AND created_at < NOW() - INTERVAL '1 hour';
    " 2>/dev/null | tr -d ' ' || echo "0")
    
    if [[ "$STUCK_COUNT" -gt 100 ]]; then
        log_message "ALERT: $STUCK_COUNT stuck renewals detected!"
        if [[ -n "$ALERT_WEBHOOK" ]]; then
            curl -X POST "$ALERT_WEBHOOK" -d "{
                \"text\": \"$STUCK_COUNT stuck renewals detected!\",
                \"level\": \"critical\"
            }" 2>/dev/null || true
        fi
    fi
fi

log_message "Health check completed"
EOF

# Churn evaluation script
sudo tee /opt/subscription/evaluate-churns.sh > /dev/null << 'EOF'
#!/bin/bash
# Evaluate subscriptions for churning

LOG_FILE="/var/log/subscription/churn-evaluation.log"
ALERT_WEBHOOK="${ALERT_WEBHOOK_URL:-}"

log_message() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOG_FILE"
}

log_message "Starting churn evaluation"

# This would call the renewal service API to evaluate churns
# For now, just log the action
log_message "Churn evaluation completed"
EOF

# Failed opt-in retry script
sudo tee /opt/subscription/retry-failed-optins.sh > /dev/null << 'EOF'
#!/bin/bash
# Retry failed opt-ins from priority queue

LOG_FILE="/var/log/subscription/retry-queue.log"

log_message() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOG_FILE"
}

log_message "Processing priority retry queue"

# This would call the renewal service API to process the retry queue
# For now, just log the action
log_message "Priority retry queue processing completed"
EOF

# Log cleanup script
sudo tee /opt/subscription/cleanup-logs.sh > /dev/null << 'EOF'
#!/bin/bash
# Clean up old log files

LOG_DIR="/var/log/subscription"
RETENTION_DAYS=30

find "$LOG_DIR" -name "*.log.*" -type f -mtime +$RETENTION_DAYS -delete
find "$LOG_DIR" -name "*.log" -type f -size +100M -exec truncate -s 50M {} \;

echo "$(date '+%Y-%m-%d %H:%M:%S') - Log cleanup completed" >> "$LOG_DIR/cleanup.log"
EOF

# Make scripts executable
sudo chmod +x /opt/subscription/*.sh
sudo chown "$SERVICE_USER:$SERVICE_GROUP" /opt/subscription/*.sh

# 10. Setup Prometheus monitoring
print_status "Setting up Prometheus monitoring..."
if [[ -d "/etc/prometheus" ]]; then
    sudo tee -a /etc/prometheus/prometheus.yml > /dev/null << EOF

  - job_name: 'renewal_worker'
    static_configs:
    - targets: ['localhost:9090']
      labels:
        service: 'renewal'
        environment: 'production'
EOF
    print_status "Prometheus configuration updated"
else
    print_warning "Prometheus directory not found, skipping configuration"
fi

# 11. Start services
print_status "Starting renewal worker service..."
sudo systemctl daemon-reload
sudo systemctl enable renewal-worker
sudo systemctl restart renewal-worker

# 12. Verify deployment
print_status "Verifying deployment..."
sleep 5

if systemctl is-active --quiet renewal-worker; then
    print_status "✓ Renewal worker is running"
else
    print_error "✗ Renewal worker failed to start"
    sudo systemctl status renewal-worker
    exit 1
fi

# 13. Check logs
print_status "Checking service logs..."
if [[ -f "$LOG_DIR/renewal-worker.log" ]]; then
    print_status "✓ Log file created: $LOG_DIR/renewal-worker.log"
else
    print_warning "Log file not found, checking systemd journal..."
    sudo journalctl -u renewal-worker --no-pager -n 10
fi

# 14. Final status
print_status "=== Deployment Complete ==="
print_status "Renewal worker service: renewal-worker"
print_status "Configuration: $CONFIG_DIR/renewal.yaml"
print_status "Logs: $LOG_DIR/renewal-worker.log"
print_status "Monitoring: http://localhost:9090/metrics"
print_status ""

print_status "Useful commands:"
print_status "  Check status: sudo systemctl status renewal-worker"
print_status "  View logs: sudo journalctl -u renewal-worker -f"
print_status "  Restart: sudo systemctl restart renewal-worker"
print_status "  Stop: sudo systemctl stop renewal-worker"
print_status ""

print_status "Next steps:"
print_status "1. Monitor the logs for any errors"
print_status "2. Verify database tables were created"
print_status "3. Test the renewal process with a sample subscription"
print_status "4. Set up alerting webhooks if needed"
print_status "5. Configure monitoring dashboards"

echo -e "${GREEN}Deployment completed successfully!${NC}" 