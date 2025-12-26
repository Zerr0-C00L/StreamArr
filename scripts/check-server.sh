#!/bin/bash

# StreamArr Pro - Check Server Status and Monitor Playback
# Quick diagnostic tool for the cloud server

SERVER="root@77.42.16.119"
REMOTE_PATH="/root/streamarr-pro"

echo "🔍 StreamArr Pro - Server Status Check"
echo "========================================"
echo ""

echo "📡 Connecting to $SERVER..."
echo ""

ssh "$SERVER" << 'ENDSSH'
    cd /root/streamarr 2>/dev/null || cd /home/streamarr 2>/dev/null || cd /opt/streamarr 2>/dev/null || echo "⚠️  Could not find streamarr directory"
    
    echo "1️⃣  Process Status:"
    echo "-------------------"
    if pgrep -f "bin/server" > /dev/null; then
        echo "✅ StreamArr server is running"
        ps aux | grep "bin/server" | grep -v grep
    elif docker ps | grep -q streamarr; then
        echo "✅ StreamArr Docker container is running"
        docker ps | grep streamarr
    else
        echo "❌ StreamArr server is NOT running"
    fi
    echo ""
    
    echo "2️⃣  Port Status:"
    echo "----------------"
    if netstat -tlnp 2>/dev/null | grep -q ":8080"; then
        echo "✅ Port 8080 is listening"
        netstat -tlnp 2>/dev/null | grep ":8080" || ss -tlnp | grep ":8080"
    else
        echo "❌ Port 8080 is NOT listening"
    fi
    echo ""
    
    echo "3️⃣  Recent Playback Activity (last 20 requests):"
    echo "-------------------------------------------------"
    if [ -f "logs/server.log" ]; then
        grep "\[PLAY\]" logs/server.log | tail -20 | while read line; do
            if echo "$line" | grep -q "✓"; then
                echo "✅ $line"
            elif echo "$line" | grep -q "❌"; then
                echo "❌ $line"
            else
                echo "   $line"
            fi
        done
        
        echo ""
        echo "Summary:"
        SUCCESS=$(grep "\[PLAY\].*✓" logs/server.log 2>/dev/null | wc -l)
        FAILED=$(grep "\[PLAY\].*❌" logs/server.log 2>/dev/null | wc -l)
        echo "  Success: $SUCCESS"
        echo "  Failed:  $FAILED"
    else
        echo "⚠️  No log file found at logs/server.log"
    fi
    echo ""
    
    echo "4️⃣  System Resources:"
    echo "---------------------"
    echo "CPU & Memory:"
    top -bn1 | grep "Cpu(s)" | sed "s/.*, *\([0-9.]*\)%* id.*/\1/" | awk '{print "  CPU Usage: " 100 - $1"%"}'
    free -h | awk 'NR==2{printf "  Memory Usage: %s/%s (%.2f%%)\n", $3,$2,$3*100/$2 }'
    echo ""
    
    echo "5️⃣  Disk Space:"
    echo "---------------"
    df -h | grep -E "/$|/home|/root" | awk '{printf "  %s: %s used of %s (%s)\n", $6, $3, $2, $5}'
    echo ""
    
    echo "6️⃣  Network Connectivity Test:"
    echo "-------------------------------"
    echo -n "  Torrentio addon: "
    if curl -s --max-time 5 "https://torrentio.strem.fun/manifest.json" > /dev/null 2>&1; then
        echo "✅ Reachable"
    else
        echo "❌ Not reachable"
    fi
    
    echo -n "  TMDB API: "
    if curl -s --max-time 5 "https://api.themoviedb.org" > /dev/null 2>&1; then
        echo "✅ Reachable"
    else
        echo "❌ Not reachable"
    fi
    echo ""
ENDSSH

echo "========================================"
echo ""
echo "💡 Tips:"
echo "   • Monitor live: ssh $SERVER 'tail -f $REMOTE_PATH/logs/server.log | grep --color \"\\[PLAY\\]\"'"
echo "   • Restart service: ssh $SERVER 'cd $REMOTE_PATH && ./scripts/deploy-fix.sh'"
echo "   • Full logs: ssh $SERVER 'tail -100 $REMOTE_PATH/logs/server.log'"
echo ""
