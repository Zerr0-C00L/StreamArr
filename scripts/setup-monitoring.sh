#!/bin/bash

# Setup Monitoring Script
# This script installs the health check as a cron job

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HEALTH_CHECK_SCRIPT="$SCRIPT_DIR/health-check.sh"

# Make health check script executable
chmod +x "$HEALTH_CHECK_SCRIPT"

echo "StreamArr Monitoring Setup"
echo "=========================="
echo ""
echo "This script will set up automatic health monitoring for StreamArr."
echo ""
echo "Available options:"
echo "1. Every 5 minutes (recommended)"
echo "2. Every 10 minutes"
echo "3. Every 15 minutes"
echo "4. Every 30 minutes"
echo "5. Every hour"
echo "6. Remove monitoring (uninstall cron job)"
echo "7. View current cron jobs"
echo ""
read -p "Enter your choice (1-7): " choice

case $choice in
    1)
        CRON_SCHEDULE="*/5 * * * *"
        INTERVAL="every 5 minutes"
        ;;
    2)
        CRON_SCHEDULE="*/10 * * * *"
        INTERVAL="every 10 minutes"
        ;;
    3)
        CRON_SCHEDULE="*/15 * * * *"
        INTERVAL="every 15 minutes"
        ;;
    4)
        CRON_SCHEDULE="*/30 * * * *"
        INTERVAL="every 30 minutes"
        ;;
    5)
        CRON_SCHEDULE="0 * * * *"
        INTERVAL="every hour"
        ;;
    6)
        echo "Removing StreamArr health check from crontab..."
        crontab -l 2>/dev/null | grep -v "health-check.sh" | crontab -
        echo "✅ Health check monitoring removed"
        exit 0
        ;;
    7)
        echo ""
        echo "Current cron jobs:"
        echo "=================="
        crontab -l 2>/dev/null || echo "No cron jobs found"
        exit 0
        ;;
    *)
        echo "❌ Invalid choice"
        exit 1
        ;;
esac

# Create the cron job
CRON_JOB="$CRON_SCHEDULE $HEALTH_CHECK_SCRIPT >> $SCRIPT_DIR/../logs/health-check.log 2>&1"

# Remove any existing StreamArr health check entries
crontab -l 2>/dev/null | grep -v "health-check.sh" | crontab -

# Add the new cron job
(crontab -l 2>/dev/null; echo "$CRON_JOB") | crontab -

echo ""
echo "✅ Health check monitoring installed successfully!"
echo ""
echo "Configuration:"
echo "  Interval: $INTERVAL"
echo "  Script: $HEALTH_CHECK_SCRIPT"
echo "  Log file: $SCRIPT_DIR/../logs/health-check.log"
echo ""
echo "The health check will:"
echo "  • Check if Docker containers are running"
echo "  • Check if containers are healthy"
echo "  • Check the HTTP health endpoint"
echo "  • Automatically restart if any check fails"
echo ""
echo "To view logs: tail -f $SCRIPT_DIR/../logs/health-check.log"
echo "To remove: Run this script again and choose option 6"
echo ""
echo "Testing health check now..."
bash "$HEALTH_CHECK_SCRIPT"
