#!/bin/bash

# StreamArr Health Check and Auto-Restart Script
# This script checks if the StreamArr containers are running and healthy
# If not, it automatically restarts them

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
LOG_FILE="$PROJECT_DIR/logs/health-check.log"
HEALTH_URL="http://localhost:8080/api/v1/health"
MAX_RETRIES=3
RETRY_DELAY=5

# Create logs directory if it doesn't exist
mkdir -p "$PROJECT_DIR/logs"

# Function to log messages
log_message() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# Function to check if Docker is running
check_docker() {
    if ! docker info >/dev/null 2>&1; then
        log_message "❌ Docker is not running"
        return 1
    fi
    return 0
}

# Function to check if containers exist
check_containers_exist() {
    local streamarr_exists=$(docker ps -a --filter "name=streamarr" --format "{{.Names}}" | grep -w "streamarr" | wc -l)
    local db_exists=$(docker ps -a --filter "name=streamarr-db" --format "{{.Names}}" | grep -w "streamarr-db" | wc -l)
    
    if [ "$streamarr_exists" -eq 0 ] || [ "$db_exists" -eq 0 ]; then
        log_message "⚠️  One or more containers do not exist"
        return 1
    fi
    return 0
}

# Function to check if containers are running
check_containers_running() {
    local streamarr_running=$(docker ps --filter "name=streamarr" --filter "status=running" --format "{{.Names}}" | grep -w "streamarr" | wc -l)
    local db_running=$(docker ps --filter "name=streamarr-db" --filter "status=running" --format "{{.Names}}" | grep -w "streamarr-db" | wc -l)
    
    if [ "$streamarr_running" -eq 0 ]; then
        log_message "❌ StreamArr container is not running"
        return 1
    fi
    
    if [ "$db_running" -eq 0 ]; then
        log_message "❌ Database container is not running"
        return 1
    fi
    
    return 0
}

# Function to check container health status
check_container_health() {
    local streamarr_health=$(docker inspect --format='{{.State.Health.Status}}' streamarr 2>/dev/null)
    local db_health=$(docker inspect --format='{{.State.Health.Status}}' streamarr-db 2>/dev/null)
    
    if [ "$streamarr_health" != "healthy" ]; then
        log_message "❌ StreamArr container is not healthy (Status: $streamarr_health)"
        return 1
    fi
    
    if [ "$db_health" != "healthy" ]; then
        log_message "❌ Database container is not healthy (Status: $db_health)"
        return 1
    fi
    
    return 0
}

# Function to check HTTP health endpoint
check_http_health() {
    local response=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$HEALTH_URL" 2>/dev/null)
    
    if [ "$response" != "200" ]; then
        log_message "❌ Health endpoint returned HTTP $response"
        return 1
    fi
    
    return 0
}

# Function to restart containers
restart_containers() {
    log_message "🔄 Attempting to restart containers..."
    
    cd "$PROJECT_DIR"
    
    # Try to stop containers gracefully
    docker-compose down 2>&1 | tee -a "$LOG_FILE"
    
    # Wait a moment
    sleep 5
    
    # Start containers
    docker-compose up -d 2>&1 | tee -a "$LOG_FILE"
    
    # Wait for containers to be ready
    log_message "⏳ Waiting for containers to be healthy..."
    local wait_time=0
    local max_wait=120
    
    while [ $wait_time -lt $max_wait ]; do
        sleep 5
        wait_time=$((wait_time + 5))
        
        if check_containers_running && check_container_health; then
            log_message "✅ Containers are running and healthy"
            return 0
        fi
        
        log_message "⏳ Still waiting... (${wait_time}s/${max_wait}s)"
    done
    
    log_message "❌ Containers failed to become healthy after ${max_wait}s"
    return 1
}

# Main health check logic
main() {
    log_message "🔍 Starting health check..."
    
    # Check if Docker is running
    if ! check_docker; then
        log_message "❌ Cannot proceed - Docker is not running"
        exit 1
    fi
    
    # Check if containers exist
    if ! check_containers_exist; then
        log_message "❌ Containers do not exist - manual setup required"
        exit 1
    fi
    
    # Check if containers are running
    if ! check_containers_running; then
        log_message "⚠️  Containers are not running - attempting restart"
        restart_containers
        exit $?
    fi
    
    # Check container health status
    if ! check_container_health; then
        log_message "⚠️  Containers are unhealthy - attempting restart"
        restart_containers
        exit $?
    fi
    
    # Check HTTP health endpoint with retries
    local retry=0
    while [ $retry -lt $MAX_RETRIES ]; do
        if check_http_health; then
            log_message "✅ All health checks passed"
            exit 0
        fi
        
        retry=$((retry + 1))
        if [ $retry -lt $MAX_RETRIES ]; then
            log_message "⚠️  Health check failed, retrying ($retry/$MAX_RETRIES)..."
            sleep $RETRY_DELAY
        fi
    done
    
    # If we got here, HTTP health check failed after retries
    log_message "❌ Health endpoint failed after $MAX_RETRIES attempts - attempting restart"
    restart_containers
    exit $?
}

# Run main function
main
