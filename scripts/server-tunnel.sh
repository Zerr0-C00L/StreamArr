#!/bin/bash
# StreamArr SOCKS Proxy via Home Router
# Runs on server to create SOCKS proxy via home IP

ROUTER_IP="${ROUTER_IP:-}"
ROUTER_PORT="${ROUTER_PORT:-22}"
ROUTER_USER="${ROUTER_USER:-}"
ROUTER_PASS="${ROUTER_PASS:-}"
SOCKS_PORT="${SOCKS_PORT:-9050}"

if [ -z "$ROUTER_IP" ] || [ -z "$ROUTER_USER" ] || [ -z "$ROUTER_PASS" ]; then
    echo "ERROR: Required environment variables not set:"
    echo "  ROUTER_IP, ROUTER_USER, ROUTER_PASS"
    exit 1
fi

echo "[$(date)] StreamArr Tunnel starting..."
echo "Router: ${ROUTER_USER}@${ROUTER_IP}:${ROUTER_PORT}"
echo "SOCKS: localhost:${SOCKS_PORT}"

while true; do
    echo "[$(date)] Creating SOCKS proxy..."
    
    sshpass -p "${ROUTER_PASS}" ssh \
        -D ${SOCKS_PORT} \
        -N \
        -o ServerAliveInterval=30 \
        -o ServerAliveCountMax=3 \
        -o StrictHostKeyChecking=no \
        -o ExitOnForwardFailure=yes \
        -p ${ROUTER_PORT} \
        ${ROUTER_USER}@${ROUTER_IP}
    
    EXIT_CODE=$?
    echo "[$(date)] Tunnel died (exit: ${EXIT_CODE}), restarting in 5s..."
    sleep 5
done
