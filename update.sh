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
            docker-compose down 2>&1 | tee -a "$LOG_FILE"
            docker-compose up -d --build 2>&1 | tee -a "$LOG_FILE"
            log "Container rebuild complete!"
        else
            log "ERROR: docker-compose.yml not found at /app/host"
            log "Please rebuild manually: cd /opt/StreamArr && docker-compose down && docker-compose up -d --build"
        fi
    else
        log "Docker socket not mounted, please rebuild manually"
        log "From host, run: cd /opt/StreamArr && docker-compose down && docker-compose up -d --build"
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

# Backup current binary
if [ -f bin/server ]; then
    cp bin/server bin/server.backup
    log "Backed up current server to bin/server.backup"
fi

# Get the PID file or find the running process
PID=""
if [ -f logs/server.pid ]; then
    PID=$(cat logs/server.pid)
fi

if [ -z "$PID" ]; then
    PID=$(pgrep -f "bin/server" 2>/dev/null || true)
fi

# Stop old server
if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    log "Stopping old server (PID: $PID)..."
    kill "$PID"
    sleep 2
fi

# Start new server
log "Starting new server..."
nohup ./bin/server > logs/server.log 2>&1 &
NEW_PID=$!
echo $NEW_PID > logs/server.pid

sleep 3

# Verify server is running
if kill -0 "$NEW_PID" 2>/dev/null; then
    log "✅ Update successful! New server running with PID: $NEW_PID"
    
    # Get new version
    VERSION=$(curl -s http://localhost:8080/api/v1/version 2>/dev/null | grep -o '"current_version":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
    log "New version: $VERSION"
else
    log "❌ Server failed to start. Restoring backup..."
    if [ -f bin/server.backup ]; then
        mv bin/server.backup bin/server
        nohup ./bin/server > logs/server.log 2>&1 &
        echo $! > logs/server.pid
        log "Restored backup and restarted."
    fi
    exit 1
fi

log "Update complete!"
