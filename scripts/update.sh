#!/bin/bash
# StreamArr Pro Auto-Update Script
# This script pulls the latest code and updates the deployment

set -e

# Check if running in Docker first
if [ -f /.dockerenv ]; then
    # In Docker, switch to mounted host directory immediately
    if [ -d /app/host ]; then
        cd /app/host
        echo "[DEBUG] Changed to /app/host directory"
    else
        echo "[ERROR] /app/host not found! Is the host directory mounted?"
        exit 1
    fi
fi

# Always work from the directory containing this script (or /app/host in Docker)
if [ ! -f /.dockerenv ]; then
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    cd "$SCRIPT_DIR/.."
fi

echo "[DEBUG] Working directory: $(pwd)"
echo "[DEBUG] Contents: $(ls -la)"

# Create logs directory first
mkdir -p logs

LOG_FILE="logs/update.log"
LOCK_FILE="logs/update.lock"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# Check for existing update process
if [ -f "$LOCK_FILE" ]; then
    LOCK_PID=$(cat "$LOCK_FILE" 2>/dev/null || echo "")
    if [ -n "$LOCK_PID" ] && kill -0 "$LOCK_PID" 2>/dev/null; then
        log "Update already in progress (PID: $LOCK_PID)"
        exit 1
    else
        log "Removing stale lock file"
        rm -f "$LOCK_FILE"
    fi
fi

# Create lock file
echo $$ > "$LOCK_FILE"
trap "rm -f $LOCK_FILE" EXIT

log "Starting StreamArr Pro update..."

# Get the branch parameter (default: main)
BRANCH="${1:-main}"

# Check if running in Docker
if [ -f /.dockerenv ]; then
    log "Running in Docker container"
    
    # Check if docker socket is mounted
    if [ -S /var/run/docker.sock ]; then
        log "Docker socket available, rebuilding containers..."
        
        # Use docker-compose from mounted location (already in /app/host)
        if [ -f docker-compose.yml ]; then
            # Fetch latest code and tags
            log "Fetching latest code and tags..."
            git fetch --all --tags --prune 2>&1 | tee -a "$LOG_FILE"
            
            # Clean up untracked files that might conflict with checkout
            log "Cleaning up untracked files that might conflict..."
            git clean -fd migrations/ 2>/dev/null || true
            git clean -fd internal/xtream/ 2>/dev/null || true
            rm -f internal/xtream/._*.go 2>/dev/null || true
            rm -f server-linux 2>/dev/null || true
            rm -f channels/combined.m3u 2>/dev/null || true
            
            # Determine if BRANCH is a tag or a branch
            if git show-ref --verify --quiet "refs/tags/$BRANCH"; then
                log "Checking out tag: $BRANCH"
                git checkout "tags/$BRANCH" 2>&1 | tee -a "$LOG_FILE"
            else
                log "Checking out branch: $BRANCH"
                git reset --hard origin/$BRANCH 2>&1 | tee -a "$LOG_FILE"
                git checkout $BRANCH 2>&1 | tee -a "$LOG_FILE" || true
                git pull origin $BRANCH 2>&1 | tee -a "$LOG_FILE"
            fi
            
            # Get version info from git after ensuring we have all tags
            VERSION=$(git describe --tags --abbrev=0 2>/dev/null || git describe --always 2>/dev/null || echo "main")
            COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
            BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
            
            log "Building version: $VERSION (commit: $COMMIT)"
            
            # Write to .env file so docker-compose picks it up
            cat > .env << EOF
VERSION=$VERSION
COMMIT=$COMMIT
BUILD_DATE=$BUILD_DATE
EOF
            
            log "Version info written to .env file"
            
            # Create a standalone update script on the HOST that will run after we exit
            # This script runs OUTSIDE the container via docker run, so it survives container stop
            UPDATE_RUNNER=".update_runner.sh"
            cat > "$UPDATE_RUNNER" << 'RUNNER_EOF'
#!/bin/sh
# Auto-generated update runner - runs on host via docker
cd /workspace
mkdir -p logs

# Get the project name from docker compose ls (most reliable)
RUNNING_PROJECT=$(docker compose ls --format json 2>/dev/null | grep -o '"Name":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -z "$RUNNING_PROJECT" ]; then
    # Fallback: derive from folder name on host (StreamArrPro -> streamarrpro)
    RUNNING_PROJECT="streamarrpro"
fi
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Using project name: $RUNNING_PROJECT" >> logs/update.log

sleep 3

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Stopping containers..." >> logs/update.log
docker compose -p "$RUNNING_PROJECT" down 2>&1 >> logs/update.log

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Cleaning up unused Docker resources..." >> logs/update.log
docker system prune -af --volumes 2>&1 >> logs/update.log || echo "[$(date '+%Y-%m-%d %H:%M:%S')] Warning: Cleanup had issues but continuing..." >> logs/update.log

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Building new image (this may take a few minutes)..." >> logs/update.log
docker compose -p "$RUNNING_PROJECT" build --no-cache --pull 2>&1 >> logs/update.log

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Starting containers..." >> logs/update.log
docker compose -p "$RUNNING_PROJECT" up -d 2>&1 >> logs/update.log

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Cleaning up unused Docker resources..." >> logs/update.log
docker system prune -af --volumes 2>&1 >> logs/update.log || echo "[$(date '+%Y-%m-%d %H:%M:%S')] Warning: Cleanup had issues but continuing..." >> logs/update.log

echo "[$(date '+%Y-%m-%d %H:%M:%S')] ✅ Container rebuild complete!" >> logs/update.log

# Cleanup
rm -f /workspace/.update_runner.sh
RUNNER_EOF
            chmod +x "$UPDATE_RUNNER"
            
            log "Starting detached rebuild process..."
            
            # Pre-pull the docker:cli image first so the update doesn't fail waiting for it
            log "Pulling docker:cli image (if needed)..."
            docker pull docker:cli >> "$LOG_FILE" 2>&1 || log "Warning: Could not pre-pull docker:cli"
            
            # Remove any existing updater container first
            docker rm -f streamarr-updater 2>/dev/null || true
            
            # Get the actual host path by inspecting the current container's mount
            # /app/host inside the container maps to the host's project directory
            HOST_PATH=$(docker inspect streamarr --format '{{range .Mounts}}{{if eq .Destination "/app/host"}}{{.Source}}{{end}}{{end}}' 2>/dev/null)
            if [ -z "$HOST_PATH" ]; then
                # Fallback to common paths
                HOST_PATH="/root/StreamArrPro"
            fi
            log "Host project path: $HOST_PATH"
            
            # Run the update script in a separate container that mounts the host directory
            # This container will survive even when we stop the main streamarr container
            # Using docker:cli which has sh and docker-compose plugin
            log "Launching update container..."
            docker run -d \
                --name streamarr-updater \
                --restart=no \
                -v /var/run/docker.sock:/var/run/docker.sock \
                -v "$HOST_PATH:/workspace" \
                -w /workspace \
                docker:cli \
                sh -c 'cd /workspace && sh .update_runner.sh' \
                >> "$LOG_FILE" 2>&1
            
            RESULT=$?
            if [ $RESULT -eq 0 ]; then
                log "✅ Update process started in background. Container will rebuild shortly."
                log "Check logs/update.log for progress."
            else
                log "❌ Failed to start updater container (exit code: $RESULT)"
                log "Please update manually: docker compose down && docker compose up -d --build"
            fi
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

# Check if Go is installed (prefer /usr/local/go/bin/go)
GO_BIN="go"
if [ -x "/usr/local/go/bin/go" ]; then
    GO_BIN="/usr/local/go/bin/go"
elif ! command -v go &> /dev/null; then
    log "ERROR: Go is not installed. Cannot build from source."
    log "Please install Go 1.21+ or update manually."
    exit 1
fi

log "Using Go binary: $GO_BIN"

# Clean up untracked files that might conflict with checkout
log "Cleaning up untracked files that might conflict..."
git clean -fd migrations/ 2>/dev/null || true
git clean -fd internal/xtream/ 2>/dev/null || true
rm -f internal/xtream/._*.go 2>/dev/null || true
rm -f server-linux 2>/dev/null || true
rm -f channels/combined.m3u 2>/dev/null || true

# Pull latest changes
log "Pulling latest changes from GitHub..."
git fetch --all --tags

# Determine if BRANCH is a tag or a branch
if git show-ref --verify --quiet "refs/tags/$BRANCH"; then
    log "Checking out tag: $BRANCH"
    git checkout "tags/$BRANCH"
else
    log "Resetting to branch: origin/$BRANCH"
    git reset --hard "origin/$BRANCH"
fi

# Get version info
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Package path for ldflags
PKG="github.com/Zerr0-C00L/StreamArr/internal/api"

# Build frontend if node is available
if command -v npm &> /dev/null; then
    log "Building frontend..."
    cd streamarr-pro-ui
    npm install >> "$LOG_FILE" 2>&1 || log "Warning: npm install failed"
    npm run build >> "$LOG_FILE" 2>&1 || log "Warning: npm build failed"
    cd ..
else
    log "Warning: npm not found, skipping frontend build"
fi

# Build new server binary with version info
log "Building new server binary..."
$GO_BIN build -ldflags "-X ${PKG}.Version=latest -X ${PKG}.Commit=${COMMIT} -X ${PKG}.BuildDate=${BUILD_DATE}" \
    -o bin/server ./cmd/server || {
    log "ERROR: Failed to build server"
    exit 1
}

# Build new worker binary with version info
log "Building new worker binary..."
$GO_BIN build -ldflags "-X ${PKG}.Version=latest -X ${PKG}.Commit=${COMMIT} -X ${PKG}.BuildDate=${BUILD_DATE}" \
    -o bin/worker ./cmd/worker || {
    log "ERROR: Failed to build worker"
    exit 1
}

# Restart services
log "Restarting services..."
systemctl restart streamarr
systemctl restart streamarr-worker

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
