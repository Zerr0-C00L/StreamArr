#!/bin/sh
# StreamArr Router Tunnel - Maintains SSH tunnel to bypass Cloudflare
# This script runs on Asus router to route Torrentio traffic through home IP

while true; do
    echo "[$(date)] Starting SSH tunnel to 77.42.16.119..."
    ssh -R 9050 -N \
        -o ServerAliveInterval=30 \
        -o ServerAliveCountMax=3 \
        -o StrictHostKeyChecking=no \
        -o ExitOnForwardFailure=yes \
        root@77.42.16.119
    
    echo "[$(date)] Tunnel died, restarting in 10 seconds..."
    sleep 10
done
