#!/bin/sh
# StreamArr Pro SSH Tunnel - Simple approach
# Creates SOCKS proxy on server that exits via router's home IP

LOG_FILE="/tmp/streamarr-tunnel.log"
PID_FILE="/tmp/streamarr-tunnel.pid"
SERVER_HOST="77.42.16.119"
SERVER_USER="root"
SOCKS_PORT="9050"

log() {
    echo "[$(date)] $1" >> "$LOG_FILE"
}

start_tunnel() {
    if [ -f "$PID_FILE" ]; then
        OLD_PID=$(cat "$PID_FILE")
        if [ -n "$OLD_PID" ] && kill -0 "$OLD_PID" 2>/dev/null; then
            log "Tunnel already running with PID $OLD_PID"
            return 0
        fi
        rm -f "$PID_FILE"
    fi

    log "Starting reverse tunnel: Server will connect to socks5://localhost:$SOCKS_PORT -> router home IP"
    # Reverse tunnel: Forward server's localhost:9050 to router's localhost:1080
    # Then create SOCKS proxy on router that the server will use
    /opt/bin/ssh -R "$SOCKS_PORT:localhost:1080" -f -N \
        -o StrictHostKeyChecking=no \
        -o ServerAliveInterval=60 \
        -o ServerAliveCountMax=3 \
        -o ExitOnForwardFailure=yes \
        -o GatewayPorts=no \
        "$SERVER_USER@$SERVER_HOST" 2>> "$LOG_FILE" &
    
    sleep 2
    PID=$(ps | grep "[s]sh -R $SOCKS_PORT" | awk '{print $1}')
    if [ -n "$PID" ]; then
        echo "$PID" > "$PID_FILE"
        log "Tunnel started with PID $PID"
        return 0
    fi
    
    log "Failed to start tunnel"
    return 1
}

stop_tunnel() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
            log "Stopping tunnel (PID $PID)..."
            kill "$PID"
            rm -f "$PID_FILE"
            log "Tunnel stopped"
        else
            rm -f "$PID_FILE"
        fi
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
