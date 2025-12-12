# StreamArr

**Self-hosted media server for Live TV, Movies & Series with Xtream Codes & M3U8 support**

Generate dynamic playlists for Live TV, Movies and TV Series using Xtream Codes compatible API. Stream content via Real-Debrid, Torrentio, Comet, MediaFusion and direct sources. Perfect for apps like Chillio,TiviMate, iMPlayer, IPTV Smarters Pro, XCIPTV and more.

[![Download ZIP](https://img.shields.io/badge/Download%20ZIP-latest-blue?style=for-the-badge&logo=github)](https://github.com/Zerr0-C00L/StreamArr/archive/refs/heads/main.zip)
[![Ko-fi](https://www.ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/zeroq)

---

## 🎬 Features

### Content Management
- **Movies & Series Library** - Add content from TMDB with full metadata
- **Movie Collections** - Auto-detect and complete movie collections (e.g., add one MCU movie, get them all)
- **Live TV** - 500+ channels with EPG support (DrewLive, DaddyLive, PlutoTV, etc.)
- **MDBList Integration** - Sync watchlists and custom lists automatically
- **Quality Profiles** - Set preferred resolution (4K, 1080p, 720p) per item

### Streaming Providers
- **Multi-Provider Fallback** - Automatically tries next provider if one fails
- **Real-Debrid** - Premium cached torrents
- **Torrentio** - Direct torrent streaming
- **Comet** - Works on datacenter IPs (Hetzner, DigitalOcean)
- **MediaFusion** - ElfHosted backup provider

### Background Services
- **Collection Sync** - Scans library and adds missing collection movies
- **MDBList Sync** - Keeps library in sync with your watchlists
- **EPG Update** - Refreshes TV guide data automatically
- **Stream Search** - Finds streams for monitored content
- **Playlist Generation** - Regenerates M3U8 playlists
- **Cache Cleanup** - Removes expired cache entries

### Modern Web UI
- **Dashboard** - Overview of library stats and recent additions
- **Library Browser** - Browse movies/series with sorting options
- **Search** - Find and add content from TMDB
- **Settings** - Configure providers, quality, services, and more
- **Services Monitor** - View background task status with manual triggers

### API Compatibility
- **Xtream Codes API** - Full compatibility with IPTV apps
- **M3U8 Playlists** - Standard playlist format support
- **REST API** - Modern JSON API for all operations

---

## 📦 Quick Start

### Prerequisites
- Go 1.21+
- PostgreSQL 14+
- Node.js 18+ (for UI development)

### Installation

\`\`\`bash
# Clone repository
git clone https://github.com/Zerr0-C00L/StreamArr.git
cd StreamArr

# Run database migrations
psql \$DATABASE_URL < migrations/001_initial_schema.up.sql
psql \$DATABASE_URL < migrations/002_add_settings.up.sql
psql \$DATABASE_URL < migrations/003_add_users.up.sql
psql \$DATABASE_URL < migrations/004_add_collections.up.sql

# Build and start
./start-all.sh
\`\`\`

### Access Points
- **Web UI**: http://localhost:8080
- **Xtream Codes**: http://localhost:8080/player_api.php

---

## 🔧 Configuration

# Optional - Streaming Providers
RD_API_KEY=your_realdebrid_key
TORRENTIO_URL=https://torrentio.strem.fun
COMET_URL=https://comet.elfhosted.com
MEDIAFUSION_URL=https://mediafusion.elfhosted.com

### Web UI Settings

Access Settings from the web UI to configure:

| Tab | Options |
|-----|---------|
| **General** | Server URL, authentication |
| **Providers** | Enable/disable and configure streaming providers |
| **Quality** | Default quality profiles, auto-add collections |
| **Live TV** | M3U sources, EPG URLs |
| **MDBList** | API key, watchlist sync |
| **Services** | View/trigger background tasks |

---

## 📱 IPTV App Setup

### Xtream Codes (Recommended)
Most IPTV apps support Xtream Codes login:

| Field | Value |
|-------|-------|
| Server | \`http://your-ip:8080\` |
| Username | \`any\` |
| Password | \`any\` |

### M3U Playlist
For apps without Xtream support:
\`\`\`
http://your-ip:8080/playlist.m3u8
\`\`\`

### Supported Apps
- Chillio
- TiviMate
- iMPlayer
- IPTV Smarters Pro
- XCIPTV Player
- OTT Navigator
- Kodi (with IPTV Simple Client)

---

## 🎯 Usage Guide

### Adding Content

1. **Search** - Use the search bar to find movies/series on TMDB
2. **Add to Library** - Click + to add with your preferred quality profile
3. **Collections** - Enable "Auto-add Collections" in Settings > Quality to automatically complete movie collections

### Library Management

- **Sorting** - Sort by title, date added, release date, rating, or year
- **Filtering** - Filter by monitored status, availability, type
- **Bulk Actions** - Select multiple items for batch operations

### Background Services

View and manage background tasks in Settings > Services:

| Service | Interval | Description |
|---------|----------|-------------|
| Collection Sync | 24 hours | Links movies to collections, adds missing titles |
| MDBList Sync | 6 hours | Syncs with configured watchlists |
| EPG Update | 6 hours | Refreshes TV guide data |
| Stream Search | 30 mins | Finds streams for monitored content |
| Playlist Generation | 12 hours | Regenerates M3U8 playlists |
| Cache Cleanup | 1 hour | Removes expired entries |
| Channel Refresh | 1 hour | Updates Live TV channel list |

Click "Run Now" to manually trigger any service.

---

## 🏗️ Architecture

\`\`\`
StreamArr/
├── cmd/
│   ├── server/          # Main API server
│   ├── worker/          # Background task worker
│   └── migrate/         # Database migration tool
├── internal/
│   ├── api/             # HTTP handlers and routes
│   ├── database/        # PostgreSQL stores
│   ├── models/          # Data models
│   ├── services/        # External service clients (TMDB, RD, etc.)
│   ├── livetv/          # Live TV channel management
│   ├── epg/             # Electronic Program Guide
│   └── settings/        # Configuration management
├── migrations/          # SQL migrations
├── streamarr-ui/        # React frontend
└── cache/               # Local cache files
\`\`\`

### Tech Stack
- **Backend**: Go 1.24
- **Frontend**: React 19 + TypeScript + Vite + TailwindCSS
- **Database**: PostgreSQL
- **API**: REST + Xtream Codes compatible

---

## 🚀 Deployment

### Docker (Coming Soon)

\`\`\`bash
docker-compose up -d
\`\`\`

### Cloud (Hetzner/DigitalOcean)

\`\`\`bash
# 1. Create Ubuntu 24.04 Server

# 2. Install dependencies
apt update && apt install -y golang postgresql nginx

# 3. Clone and configure
git clone https://github.com/Zerr0-C00L/StreamArr.git /var/www/streamarr
cd /var/www/streamarr
cp .env.example .env
nano .env  # Add your API keys

# 4. Setup database
sudo -u postgres createdb streamarr
# Run migrations...

# 5. Build and run
./start-all.sh

# 6. (Optional) Setup systemd service for auto-start
\`\`\`

### Reverse Proxy (nginx)

\`\`\`nginx
server {
    listen 80;
    server_name streamarr.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
    }
}
\`\`\`

---

## 📝 API Reference

### REST API

| Endpoint | Method | Description |
|----------|--------|-------------|
| \`/api/v1/movies\` | GET | List library movies |
| \`/api/v1/movies\` | POST | Add movie to library |
| \`/api/v1/series\` | GET | List library series |
| \`/api/v1/series\` | POST | Add series to library |
| \`/api/v1/search/movies\` | GET | Search TMDB for movies |
| \`/api/v1/search/series\` | GET | Search TMDB for series |
| \`/api/v1/collections\` | GET | List collections |
| \`/api/v1/services\` | GET | Get background service status |
| \`/api/v1/services/{name}/trigger\` | POST | Trigger a service |
| \`/api/v1/settings\` | GET/PUT | Get/update settings |
| \`/api/v1/livetv/channels\` | GET | List Live TV channels |

### Xtream Codes API

| Endpoint | Description |
|----------|-------------|
| \`/player_api.php?action=get_live_categories\` | Live TV categories |
| \`/player_api.php?action=get_live_streams\` | Live TV channels |
| \`/player_api.php?action=get_vod_categories\` | Movie categories |
| \`/player_api.php?action=get_vod_streams\` | Movie list |
| \`/player_api.php?action=get_series_categories\` | Series categories |
| \`/player_api.php?action=get_series\` | Series list |

---

## 🔄 Changelog

### December 12, 2025
- **Movie Collections** - Auto-detect collections, add missing movies
- **Services Monitor** - View/trigger background tasks from UI
- **Library Sorting** - 10 sort options (title, added, release, rating, year)
- **Collection Badges** - Visual indicator on movie cards

### December 8, 2025
- **Multi-Provider Support** - Comet, MediaFusion, Torrentio fallback
- **Cloud Deployment** - Optimized for datacenter hosting
- **Background Sync** - Worker daemon for automatic updates
- **Full Go Rewrite** - Improved performance and reliability

### September 28, 2025
- Fixed Live TV and added DrewLive (7,000+ channels)
- Fixed Real-Debrid cache checks
- Fixed Adult VOD (10K movies)
- Major HeadlessVidX overhaul

---

## ⚠️ Legal Disclaimer

This software retrieves movie information from TMDB and searches for content on third-party sources. The legality of streaming content through these sources varies by jurisdiction. Users are responsible for ensuring compliance with local laws. Always respect copyright and terms of service.

---

## 🤝 Contributing

Contributions welcome! Please open an issue or PR.

## 📄 License

MIT License - see [LICENSE.md](LICENSE.md)
