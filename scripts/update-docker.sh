#!/bin/bash
# StreamArr Pro Docker Auto-Update Script
# This script pulls the latest code and rebuilds the container

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

LOG_FILE="logs/update.log"
mkdir -p logs

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "Starting StreamArr Pro Docker update..."

# Check if running in a container
if [ -f /.dockerenv ]; then
    log "Running in Docker container"
    
    # Check if docker-compose is available
    if ! command -v docker-compose &> /dev/null; then
        log "ERROR: docker-compose is not available"
        exit 1
    fi
    
    # Pull latest changes
    log "Pulling latest changes from GitHub..."
    git fetch origin main
    git reset --hard origin/main
    
    # Rebuild and restart containers
    log "Rebuilding and restarting containers..."
    docker-compose down
    docker-compose up -d --build
    
    log "Update complete!"
else
    log "Not running in Docker, falling back to standard update..."
    exec ./update.sh "$@"
fi
