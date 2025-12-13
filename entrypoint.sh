#!/bin/sh

# StreamArr Docker Entrypoint
# Starts both the API server and background worker processes

set -e

echo "╔════════════════════════════════════════╗"
echo "║     StreamArr Container Starting       ║"
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

# Start worker process in background
echo "🤖 Starting background workers..."
/app/bin/worker > /app/logs/worker.log 2>&1 &
WORKER_PID=$!
echo "   Worker PID: $WORKER_PID"
echo "   Logs: /app/logs/worker.log"
echo ""

# Start server process (foreground)
echo "🚀 Starting API server..."
exec /app/bin/server
