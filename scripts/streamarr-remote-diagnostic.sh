#!/bin/bash
# StreamArr Remote Diagnostic Script
# Run this on the remote server: bash streamarr-remote-diagnostic.sh

echo "=== StreamArr Remote Server Diagnostic ==="
echo ""

# Find docker command
DOCKER_CMD=$(which docker || find /usr -name docker 2>/dev/null | head -1)
if [ -z "$DOCKER_CMD" ]; then
    echo "❌ Docker not found!"
    exit 1
fi
echo "✓ Docker found at: $DOCKER_CMD"

# Get container status
echo ""
echo "1. Container Status:"
$DOCKER_CMD ps | grep -E "streamarr|postgres"

echo ""
echo "2. Recent StreamArr Logs (last 30 lines):"
$DOCKER_CMD logs streamarr --tail 30 2>&1

echo ""
echo "3. Database Status Check:"
$DOCKER_CMD exec streamarr-db psql -U streamarr -d streamarr -c "
  SELECT 
    'Total Movies' as metric, COUNT(*) as count FROM library_movies
  UNION ALL
  SELECT 'Total Cached Streams' as metric, COUNT(*) as count FROM media_streams
  UNION ALL
  SELECT 'Movies with IMDB ID' as metric, COUNT(*) as count FROM library_movies WHERE metadata->>'imdb_id' IS NOT NULL
;" 2>&1

echo ""
echo "4. Sample of Movies in Database (first 5):"
$DOCKER_CMD exec streamarr-db psql -U streamarr -d streamarr -c "
  SELECT id, title, metadata->>'imdb_id' as imdb_id 
  FROM library_movies 
  LIMIT 5;
" 2>&1

echo ""
echo "5. Cache Scanner Service Status:"
$DOCKER_CMD logs streamarr 2>&1 | grep -i "cache\|scanner" | tail -20

echo ""
echo "=== End Diagnostic ==="
