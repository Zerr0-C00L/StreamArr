#!/bin/bash

# StreamArr Pro - Stop All Services
echo "╔════════════════════════════════════════╗"
echo "║     Stopping StreamArr Pro Services    ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Stop backend services
echo "🛑 Stopping backend services..."
./stop.sh

# Stop frontend
echo ""
echo "🛑 Stopping frontend development server..."
pkill -f "vite"

echo ""
echo "✅ All services stopped!"
