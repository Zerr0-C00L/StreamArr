#!/bin/bash
# Load proxies from proxies.txt and format them for TORRENTIO_PROXY env var

PROXY_FILE="/app/proxies.txt"

if [ ! -f "$PROXY_FILE" ]; then
    echo "Proxy file not found at $PROXY_FILE"
    exit 0
fi

# Convert proxies from "IP:PORT:USER:PASS" to "http://USER:PASS@IP:PORT" format
PROXIES=""
while IFS=: read -r ip port user pass; do
    # Skip empty lines
    [ -z "$ip" ] && continue
    
    # Build proxy URL
    PROXY_URL="http://${user}:${pass}@${ip}:${port}"
    
    # Append to list (comma-separated)
    if [ -z "$PROXIES" ]; then
        PROXIES="$PROXY_URL"
    else
        PROXIES="$PROXIES,$PROXY_URL"
    fi
done < "$PROXY_FILE"

# Export for use by server
export TORRENTIO_PROXY="$PROXIES"

echo "Loaded $(echo $PROXIES | tr ',' '\n' | wc -l) proxies for Torrentio/TorrentDB"
