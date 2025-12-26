#!/bin/bash

# Script to check movie stream fetching diagnostics
# Usage: ./scripts/check-movie-streams.sh [movie_title_search]
# Or just run without arguments to see recent stream fetch diagnostics

SERVER="root@77.42.16.119"

if [ -z "$1" ]; then
    echo "=================================="
    echo "📋 Recent Stream Fetch Diagnostics"
    echo "=================================="
    echo ""
    echo "Showing logs from recent stream fetches..."
    echo "💡 TIP: Click on a movie in the web UI to trigger a fresh fetch"
    echo ""
    ssh $SERVER "docker logs streamarr --tail 200 2>&1 | grep -E '\[DEBUG\]|\[STREAM-FETCH\]|\[YEAR-FILTER\]|\[PROVIDER\]|\[EPISODE-FILTER\]|\[YEAR-EXTRACT\]|ERROR.*stream|WARNING.*IMDB|Fetching streams for movie' | tail -50"
    echo ""
    echo "=================================="
    echo "Key Indicators:"
    echo "  [DEBUG] - Movie metadata and IMDB ID"
    echo "  [STREAM-FETCH] - Stream fetching process"
    echo "  [PROVIDER] - Which providers returned streams"
    echo "  [YEAR-FILTER] - Year-based filtering details"
    echo "  [EPISODE-FILTER] - Episode streams being filtered"
    echo "=================================="
else
    SEARCH_TERM="$1"
    echo "=================================="
    echo "🔍 Searching for: $SEARCH_TERM"
    echo "=================================="
    echo ""
    ssh $SERVER "docker logs streamarr --tail 500 2>&1 | grep -i \"$SEARCH_TERM\" | grep -E 'streams|IMDB|FILTER|PROVIDER' | tail -30"
fi
