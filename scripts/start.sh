#!/bin/bash

# StreamArr Pro Quick Start Script
# Quickly sets up and starts the StreamArr Pro system

set -e

echo "╔════════════════════════════════════════╗"
echo "║       StreamArr Pro Quick Start        ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Default database URL (can be overridden via environment)
DATABASE_URL="${DATABASE_URL:-postgres://streamarr:streamarr_password@localhost:5432/streamarr?sslmode=disable}"
SERVER_PORT="${SERVER_PORT:-8080}"

# Check if binaries exist
if [ ! -f bin/server ] || [ ! -f bin/worker ] || [ ! -f bin/migrate ]; then
    echo "🔨 Building binaries..."
    go build -o bin/server cmd/server/main.go
    go build -o bin/worker cmd/worker/main.go
    go build -o bin/migrate cmd/migrate/main.go
    echo "✅ Binaries built successfully"
    echo ""
fi

# Check database connection
echo "🔍 Checking database connection..."
if psql "$DATABASE_URL" -c "SELECT 1" &>/dev/null; then
    echo "✅ Database connection successful"
else
    echo "❌ Cannot connect to database!"
    echo ""
    echo "Options:"
    echo "  1. Run: ./setup_database.sh"
    echo "  2. Or ensure PostgreSQL is running"
    echo "  3. Set DATABASE_URL environment variable if using custom database"
    exit 1
fi

# Check if migrations are needed
echo ""
echo "🗄️  Checking database schema..."
TABLE_COUNT=$(psql "$DATABASE_URL" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public';" 2>/dev/null | tr -d ' ' || echo "0")

if [ "$TABLE_COUNT" -lt 6 ]; then
    echo "📦 Running migrations..."
    DATABASE_URL="$DATABASE_URL" ./bin/migrate up
    echo "✅ Migrations applied"
else
    echo "✅ Database schema is up to date"
fi

# Display configuration info
echo ""
echo "🔑 Configuration:"
echo "   All settings are managed via the Web UI (Settings page)"
echo "   API Keys, Service URLs, and other options can be set there."
echo ""

echo ""
echo "╔════════════════════════════════════════╗"
echo "║      Starting StreamArr Pro Services   ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Kill existing processes
pkill -f "bin/server" 2>/dev/null || true
pkill -f "bin/worker" 2>/dev/null || true

# Start server in background with DATABASE_URL
echo "🚀 Starting API Server on port ${SERVER_PORT}..."
DATABASE_URL="$DATABASE_URL" nohup ./bin/server > logs/server.log 2>&1 &
SERVER_PID=$!
echo "   PID: $SERVER_PID"

# Wait for server to start
sleep 2

# Test health endpoint
if curl -s http://localhost:${SERVER_PORT}/api/v1/health | grep -q "ok"; then
    echo "✅ API Server is running"
else
    echo "⚠️  API Server may not be responding"
fi

# Start worker in background
echo ""
echo "⚙️  Starting Background Worker..."
DATABASE_URL="$DATABASE_URL" nohup ./bin/worker > logs/worker.log 2>&1 &
WORKER_PID=$!
echo "   PID: $WORKER_PID"

# Save PIDs
echo $SERVER_PID > logs/server.pid
echo $WORKER_PID > logs/worker.pid

echo ""
echo "╔════════════════════════════════════════╗"
echo "║        StreamArr Pro is Ready!         ║"
echo "╚════════════════════════════════════════╝"
echo ""
echo "📍 API Endpoints:"
echo "   Health Check:  http://localhost:${SERVER_PORT}/api/v1/health"
echo "   Movies:        http://localhost:${SERVER_PORT}/api/v1/movies"
echo "   Search:        http://localhost:${SERVER_PORT}/api/v1/search/movies?q=query"
echo ""
echo "⚙️  Configuration:"
echo "   Settings:      http://localhost:${SERVER_PORT} (Web UI)"
echo "   All API keys and options are configured via the Settings page"
echo ""
echo "📊 Monitoring:"
echo "   Server logs:   tail -f logs/server.log"
echo "   Worker logs:   tail -f logs/worker.log"
echo ""
echo "🛑 To stop services:"
echo "   ./stop.sh"
echo ""

# Test API
echo "🧪 Quick API Test:"
echo ""
curl -s http://localhost:${SERVER_PORT}/api/v1/health | jq '.' 2>/dev/null || curl -s http://localhost:${SERVER_PORT}/api/v1/health
echo ""
