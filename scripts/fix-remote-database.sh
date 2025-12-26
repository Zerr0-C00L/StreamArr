#!/bin/bash

# Fix Remote Database and Restart Server
# This script:
# 1. Resets the streamarr database password to match the code default
# 2. Sets DATABASE_URL environment variable
# 3. Restarts the server

set -e

TARGET_HOST="${1:-77.42.16.119}"
TARGET_USER="${2:-root}"

echo "🔧 Fixing database connection on $TARGET_HOST..."

ssh $TARGET_USER@$TARGET_HOST << 'EOF'

echo "⏸️  Stopping server..."
pkill -f "bin/server" 2>/dev/null || true
sleep 2

echo "🔑 Resetting Postgres password to match code default..."
sudo -u postgres psql -c "ALTER USER streamarr WITH PASSWORD 'streamarr_password';" 2>&1 | grep -i "alter\|error" || true

echo "📝 Setting DATABASE_URL environment variable..."
mkdir -p /root/streamarr-pro
cat > /root/streamarr-pro/.env << 'ENVFILE'
VERSION=v1.1.0
COMMIT=38bcefb
BUILD_DATE=2025-12-17T09:59:13Z
DATABASE_URL=postgres://streamarr:streamarr_password@localhost:5432/streamarr?sslmode=disable
ENVFILE

echo "✅ Environment configured"

echo "🚀 Starting server..."
cd /root/streamarr-pro
nohup ./bin/server > server.log 2>&1 &
SERVER_PID=$!
sleep 3

echo "⏳ Waiting for server to start and connect to database..."
sleep 5

echo "📋 Checking server logs..."
tail -30 server.log

echo ""
echo "✅ Database and server configured!"
echo ""
echo "Next steps:"
echo "1. Check if playlist is still working: http://77.42.16.119/player_api.php?action=get_live_categories_and_vod&username=user&password=pass&type=m3u"
echo "2. If needed, trigger Stream Search from web UI"
echo "3. Check cache status: SELECT COUNT(*) FROM media_streams;"

EOF

echo "✅ Done!"
