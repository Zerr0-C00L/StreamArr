#!/bin/bash

# StreamArr Pro - Start All Services
echo "╔════════════════════════════════════════╗"
echo "║     Starting StreamArr Pro Services    ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Stop any existing services first
echo "🧹 Cleaning up existing services..."
./stop.sh 2>/dev/null
pkill -f "vite" 2>/dev/null
sleep 1

# Start backend services in background
echo "�� Starting backend services..."
./start.sh > /dev/null 2>&1

# Wait for backend to be ready
echo "⏳ Waiting for backend to initialize..."
sleep 3

# Check if backend is running
if curl -s http://localhost:8080/api/v1/health > /dev/null 2>&1; then
    echo "✅ Backend is ready!"
else
    echo "⚠️  Backend may still be starting..."
fi

# Start frontend
echo ""
echo "🎨 Starting frontend development server..."
cd streamarr-pro-ui && npm run dev > /dev/null 2>&1 &

# Wait a moment for frontend to start
sleep 2

echo ""
echo "✅ All services started!"
echo ""
echo "╔════════════════════════════════════════╗"
echo "║           Access Points                ║"
echo "╚════════════════════════════════════════╝"
echo "   Frontend:  http://localhost:3000"
echo "   Backend:   http://localhost:8080"
echo ""
echo "📊 Monitor logs:"
echo "   tail -f logs/server.log"
echo "   tail -f logs/worker.log"
echo ""
echo "🛑 To stop all services:"
echo "   ./stop-all.sh"
