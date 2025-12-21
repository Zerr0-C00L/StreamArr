#!/bin/sh

# StreamArr Pro Docker Entrypoint
# Starts both the API server and background worker processes

set -e

echo "╔════════════════════════════════════════╗"
echo "║   StreamArr Pro Container Starting     ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Wait for database to be ready (database service is healthy in docker-compose)
echo "⏳ Waiting for database..."
sleep 5
echo "✅ Database should be ready"
echo ""

# Run database migrations
echo "🔄 Running database migrations..."
if /app/bin/migrate up 2>&1 | grep -q "Migration completed successfully\|no change"; then
    echo "✅ Migrations complete"
else
    echo "⚠️  Migration check - database may already be up to date"
fi
echo ""

# NOTE: Workers are now integrated into the server process
# No need to start a separate worker process
echo "ℹ️  Background workers are integrated into the server process"
echo ""

# Determine which server binary to use
# Always prefer container binary for stability, unless host binary is explicitly marked for hot reload
SERVER_BIN=/app/bin/server
if [ -x /app/host/bin/server-linux ] && [ -f /app/host/.hotreload ]; then
    SERVER_BIN=/app/host/bin/server-linux
    echo "🔄 Using host server binary (hot reload mode)"
else
    echo "📦 Using container server binary"
fi

# Start server process (foreground)
echo "🚀 Starting API server..."
exec $SERVER_BIN
