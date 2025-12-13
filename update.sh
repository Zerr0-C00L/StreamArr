#!/bin/bash
# StreamArr Pro Auto-Update Script
# This script pulls the latest code and updates the deployment

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

LOG_FILE="logs/update.log"
mkdir -p logs

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "Starting StreamArr Pro update..."

# Pull latest changes
log "Pulling latest changes from GitHub..."
git fetch origin main 2>&1 | tee -a "$LOG_FILE"

# Get the branch parameter (default: main)
BRANCH="${1:-main}"
log "Checking out branch: $BRANCH"
git reset --hard origin/$BRANCH 2>&1 | tee -a "$LOG_FILE"

# Check if running in Docker
if [ -f /.dockerenv ]; then
    log "Running in Docker container"
    
    # Check if docker socket is mounted
    if [ -S /var/run/docker.sock ]; then
        log "Docker socket available, rebuilding containers..."
        
        # Use docker-compose from mounted location
        if [ -f /app/host/docker-compose.yml ]; then
            cd /app/host
            
            # Get version info from git
            export VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "main")
            export COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
            export BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
            
            log "Building version: $VERSION (commit: $COMMIT)"
            
            docker-compose down 2>&1 | tee -a "$LOG_FILE"
            docker-compose up -d --build 2>&1 | tee -a "$LOG_FILE"
            log "Container rebuild complete!"
        else
            log "ERROR: docker-compose.yml not found at /app/host"
            log "Please rebuild manually: cd /opt/StreamArr_Pro && docker-compose down && docker-compose up -d --build"
        fi
    else
        log "Docker socket not mounted, please rebuild manually"
        log "From host, run: cd /opt/StreamArr_Pro && docker-compose down && docker-compose up -d --build"
    fi
    
    exit 0
fi

# Non-Docker update path
log "Running in non-Docker environment"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    log "ERROR: Go is not installed. Cannot build from source."
    log "Please install Go 1.21+ or update manually."
    exit 1
fi

# Pull latest changes
log "Pulling latest changes from GitHub..."
git fetch origin main
git reset --hard origin/main

# Build UI
log "Building UI..."
cd streamarr-pro-ui
npm install --silent
npm run build
cd ..

# Build new binary
log "Building new server..."
./build.sh

# Stop all services
log "Stopping services..."
./stop.sh 2>&1 | tee -a "$LOG_FILE"

sleep 2

# Start all services with new binary
log "Starting services..."
./start.sh 2>&1 | tee -a "$LOG_FILE"

sleep 3

# Verify server is running
if [ -f logs/server.pid ] && kill -0 "$(cat logs/server.pid)" 2>/dev/null; then
    log "✅ Update successful! Services are running"
    
    # Get new version
    VERSION=$(curl -s http://localhost:8080/api/v1/version 2>/dev/null | grep -o '"current_version":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
    log "New version: $VERSION"
else
    log "❌ Server failed to start"
    exit 1
fi

log "Update complete!"
