#!/bin/bash

# StreamArr Pro Stop Script
# Stops all StreamArr Pro services

echo "Stopping StreamArr Pro services..."

# Kill processes by PID files
if [ -f logs/server.pid ]; then
    SERVER_PID=$(cat logs/server.pid)
    if kill -0 $SERVER_PID 2>/dev/null; then
        echo "Stopping API Server (PID: $SERVER_PID)..."
        kill $SERVER_PID
        rm logs/server.pid
    fi
fi

if [ -f logs/worker.pid ]; then
    WORKER_PID=$(cat logs/worker.pid)
    if kill -0 $WORKER_PID 2>/dev/null; then
        echo "Stopping Background Worker (PID: $WORKER_PID)..."
        kill $WORKER_PID
        rm logs/worker.pid
    fi
fi

# Fallback: kill by name
pkill -f "bin/server" 2>/dev/null || true
pkill -f "bin/worker" 2>/dev/null || true

echo "✅ StreamArr Pro services stopped"
