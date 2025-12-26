#!/bin/bash

# Stream Cache Monitor Status & Toggle Script
# Usage: ./scripts/cache-monitor-status.sh [set-full|set-cached]

SERVER="77.42.16.119"
DB_USER="streamarr"
DB_PASS="streamarr_password"
DB_HOST="localhost"
DB_NAME="streamarr"

XTREAM_USER="streamarr"
XTREAM_PASS="streamarr"
M3U_URL="http://$SERVER/get.php?username=$XTREAM_USER&password=$XTREAM_PASS&type=m3u"

# Color codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to execute remote DB query
query_db() {
    ssh root@$SERVER "PGPASSWORD='$DB_PASS' psql -U $DB_USER -d $DB_NAME -h $DB_HOST -c \"$1;\" 2>/dev/null"
}

# Function to get M3U count
get_m3u_counts() {
    local total=$(curl -s "$M3U_URL" 2>/dev/null | grep -c 'EXTINF')
    local movies=$(curl -s "$M3U_URL" 2>/dev/null | grep 'group-title="Movies"' | wc -l)
    local series=$(curl -s "$M3U_URL" 2>/dev/null | grep 'group-title="Series"' | wc -l)
    local livetv=$(curl -s "$M3U_URL" 2>/dev/null | grep 'group-title="Live TV"' | wc -l)
    
    echo "$total|$movies|$series|$livetv"
}

# Main logic
case "${1:-status}" in
    "status")
        echo -e "${BLUE}════════════════════════════════════════════════════${NC}"
        echo -e "${BLUE}  Stream Cache Monitor Status${NC}"
        echo -e "${BLUE}════════════════════════════════════════════════════${NC}"
        
        # Get current setting
        SETTING=$(query_db "SELECT value FROM settings WHERE key = 'only_cached_streams'")
        MODE=$(echo "$SETTING" | tr -d ' ' | head -1)
        
        if [ "$MODE" = "true" ]; then
            echo -e "${YELLOW}Current Mode: CACHED-ONLY${NC}"
        else
            echo -e "${GREEN}Current Mode: FULL LIBRARY${NC}"
        fi
        
        echo ""
        echo -e "${BLUE}Database Content:${NC}"
        query_db "SELECT (SELECT COUNT(*) FROM library_movies WHERE monitored=true)::text || ' movies, ' || (SELECT COUNT(*) FROM library_series WHERE monitored=true)::text || ' series, ' || (SELECT COUNT(*) FROM media_streams)::text || ' cached' FROM settings LIMIT 1"
        
        echo ""
        echo -e "${BLUE}M3U Playlist Entries:${NC}"
        IFS='|' read total movies series livetv <<< "$(get_m3u_counts)"
        echo "  Total: $total (Movies: $movies, Series: $series, Live TV: $livetv)"
        
        echo ""
        echo -e "${BLUE}Usage:${NC}"
        echo "  ./cache-monitor-status.sh set-full     # Switch to FULL LIBRARY mode"
        echo "  ./cache-monitor-status.sh set-cached   # Switch to CACHED-ONLY mode"
        echo "  ./cache-monitor-status.sh toggle       # Toggle between modes"
        ;;
    
    "set-full"|"set-false")
        echo -e "${GREEN}Switching to FULL LIBRARY mode...${NC}"
        query_db "UPDATE settings SET value = 'false' WHERE key = 'only_cached_streams'"
        sleep 2
        IFS='|' read total movies series livetv <<< "$(get_m3u_counts)"
        echo -e "${GREEN}✓ M3U now has $total entries (all monitored content)${NC}"
        ;;
    
    "set-cached"|"set-true")
        echo -e "${YELLOW}Switching to CACHED-ONLY mode...${NC}"
        query_db "UPDATE settings SET value = 'true' WHERE key = 'only_cached_streams'"
        sleep 2
        IFS='|' read total movies series livetv <<< "$(get_m3u_counts)"
        echo -e "${YELLOW}✓ M3U now has $total entries (cached streams only)${NC}"
        ;;
    
    "toggle")
        SETTING=$(query_db "SELECT value FROM settings WHERE key = 'only_cached_streams'")
        MODE=$(echo "$SETTING" | tr -d ' ' | head -1)
        
        if [ "$MODE" = "true" ]; then
            echo -e "${GREEN}Toggling from CACHED-ONLY to FULL LIBRARY...${NC}"
            query_db "UPDATE settings SET value = 'false' WHERE key = 'only_cached_streams'"
            sleep 2
            IFS='|' read total movies series livetv <<< "$(get_m3u_counts)"
            echo -e "${GREEN}✓ Switched to FULL LIBRARY: $total entries${NC}"
        else
            echo -e "${YELLOW}Toggling from FULL LIBRARY to CACHED-ONLY...${NC}"
            query_db "UPDATE settings SET value = 'true' WHERE key = 'only_cached_streams'"
            sleep 2
            IFS='|' read total movies series livetv <<< "$(get_m3u_counts)"
            echo -e "${YELLOW}✓ Switched to CACHED-ONLY: $total entries${NC}"
        fi
        ;;
    
    "help"|"-h"|"--help")
        echo "Stream Cache Monitor Status & Toggle Script"
        echo ""
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  status          Show current mode and statistics (default)"
        echo "  set-full        Switch to FULL LIBRARY mode"
        echo "  set-cached      Switch to CACHED-ONLY mode"
        echo "  toggle          Toggle between modes"
        echo "  help            Show this help message"
        ;;
    
    *)
        echo "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
