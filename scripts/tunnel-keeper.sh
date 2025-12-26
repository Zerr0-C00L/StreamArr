#!/bin/bash
# StreamArr Tunnel Keeper - Run on Mac to maintain server tunnel
# Keeps SSH tunnel alive so server can bypass Cloudflare

echo "🚀 StreamArr Tunnel Keeper starting..."
echo "This will route server traffic through your home IP"
echo ""

while true; do
    echo "[$(date)] 🔄 Starting tunnel: Mac -> Server (77.42.16.119:9050)"
    
    ssh -R 9050 \
        -N \
        -o ServerAliveInterval=30 \
        -o ServerAliveCountMax=3 \
        -o ExitOnForwardFailure=yes \
        -o TCPKeepAlive=yes \
        root@77.42.16.119
    
    EXIT_CODE=$?
    echo "[$(date)] ⚠️  Tunnel died (exit code: $EXIT_CODE), restarting in 5 seconds..."
    sleep 5
done
