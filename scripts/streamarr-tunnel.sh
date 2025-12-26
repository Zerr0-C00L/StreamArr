#!/bin/sh
# StreamArr Pro Reverse SSH Tunnel
# Forwards server's localhost:9050 -> router's localhost:1080 (3proxy SOCKS)

LOG_FILE="/tmp/streamarr-tunnel.log"
PID_FILE="/tmp/streamarr-tunnel.pid"
SERVER_HOST="${SERVER_HOST:-}"
SERVER_USER="${SERVER_USER:-root}"
SOCKS_PORT="${SOCKS_PORT:-9051}"
ROUTER_SOCKS_PORT="${ROUTER_SOCKS_PORT:-1080}"

log() {
    echo "[$(date)] $1" >> "$LOG_FILE"
}

if [ -z "$SERVER_HOST" ]; then
    echo "ERROR: SERVER_HOST environment variable not set"
    exit 1
fi

start_tunnel() {
    if [ -f "$PID_FILE" ]; then
        OLD_PID=$(cat "$PID_FILE")
        if [ -n "$OLD_PID" ] && kill -0 "$OLD_PID" 2>/dev/null; then
            log "Tunnel already running with PID $OLD_PID"
            return 0
        fi
        rm -f "$PID_FILE"
    fi

    # Ensure 3proxy is running
    if ! ps | grep -v grep | grep -q "/opt/bin/3proxy"; then
        log "Starting 3proxy SOCKS server..."
        /opt/bin/3proxy /opt/etc/3proxy.cfg &
        sleep 2
    fi

    log "Creating reverse tunnel: server:$SOCKS_PORT -> router:$ROUTER_SOCKS_PORT"
    /opt/bin/ssh -R "$SOCKS_PORT:127.0.0.1:$ROUTER_SOCKS_PORT" -f -N \
        -o StrictHostKeyChecking=no \
        -o ServerAliveInterval=60 \
        -o ServerAliveCountMax=3 \
        -o ExitOnForwardFailure=yes \
        -o GatewayPorts=no \
        "$SERVER_USER@$SERVER_HOST" 2>> "$LOG_FILE"
    
    if [ $? -eq 0 ]; then
        sleep 2
        PID=$(ps | grep "[s]sh -R $SOCKS_PORT" | awk '{print $1}')
        if [ -n "$PID" ]; then
            echo "$PID" > "$PID_FILE"
            log "✓ Tunnel started with PID $PID"
            log "Server can now use: socks5://localhost:$SOCKS_PORT"
            return 0
        fi
    fi
    
    log "✗ Failed to start tunnel"
    return 1
}

stop_tunnel() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
            log "Stopping tunnel (PID $PID)..."
            kill "$PID"
        fi
        rm -f "$PID_FILE"
        log "Tunnel stopped"
    fi
}

check_tunnel() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
            echo "Tunnel is running (PID $PID)"
            return 0
        else
            echo "Tunnel is not running (stale PID file)"
            rm -f "$PID_FILE"
            return 1
        fi
    else
        echo "Tunnel is not running"
        return 1
    fi
}

case "$1" in
    start) start_tunnel ;;
    stop) stop_tunnel ;;
    restart) stop_tunnel; sleep 2; start_tunnel ;;
    status) check_tunnel ;;
    *) echo "Usage: $0 {start|stop|restart|status}"; exit 1 ;;
esac
