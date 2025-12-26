#!/bin/bash
# StreamArr Cache Diagnostic Script
# Run this to diagnose cache and playlist issues

echo "=== StreamArr Cache Monitor Diagnostic ==="
echo ""
echo "Server URL: ${SERVER_URL:-http://localhost:8888}"
echo "Username: ${USERNAME:-streamarr}"
echo "Password: ${PASSWORD:-streamarr}"
echo ""

SERVER=${SERVER_URL:-http://localhost:8888}
USER=${USERNAME:-streamarr}
PASS=${PASSWORD:-streamarr}

echo "1. Testing basic connectivity..."
if curl -s "$SERVER/health" > /dev/null; then
    echo "✓ Server is online"
else
    echo "✗ Server is offline or unreachable"
    exit 1
fi

echo ""
echo "2. Checking Xtream API responses..."

echo "  - Testing Xtream player_api.php (get_vod_streams)..."
RESPONSE=$(curl -s "$SERVER/player_api.php?action=get_vod_streams&username=$USER&password=$PASS" | head -c 200)
if [ -z "$RESPONSE" ]; then
    echo "    ✗ No response from get_vod_streams"
else
    echo "    ✓ Response received (first 200 chars):"
    echo "    $RESPONSE"
fi

echo ""
echo "3. Checking M3U playlist generation..."
echo "  - Testing /get.php endpoint..."
M3U=$(curl -s "$SERVER/get.php?username=$USER&password=$PASS" | head -20)
if echo "$M3U" | grep -q "EXTM3U"; then
    echo "    ✓ M3U playlist generated successfully"
    echo "    First 5 entries:"
    echo "$M3U" | grep -v "^#" | head -5
else
    echo "    ✗ M3U playlist generation failed"
    echo "    Response: $M3U"
fi

echo ""
echo "4. Checking cache monitor endpoint..."
CACHE=$(curl -s "$SERVER/api/v1/streams/cache/stats")
if echo "$CACHE" | grep -q "available"; then
    echo "    ✓ Cache stats endpoint working:"
    echo "    $CACHE" | jq '.' 2>/dev/null || echo "    $CACHE"
else
    echo "    ✗ Cache stats endpoint not responding properly"
fi

echo ""
echo "5. Checking cache list..."
LIST=$(curl -s "$SERVER/api/v1/streams/cache/list?type=movies" | wc -l)
echo "    Cache list response size: $LIST lines"

echo ""
echo "6. Triggering manual cache scan..."
SCAN=$(curl -s -X POST "$SERVER/api/v1/streams/cache/scan")
echo "    Response: $(echo $SCAN | head -100)"

echo ""
echo "=== Diagnostics Complete ==="
echo ""
echo "Next steps:"
echo "1. Check server logs for [CACHE-SCANNER] messages"
echo "2. Verify database has content: SELECT COUNT(*) FROM library_movies;"
echo "3. Check if streams are being cached: SELECT COUNT(*) FROM media_streams;"
echo "4. Review settings: Only Cached Streams should be OFF to see all movies"
