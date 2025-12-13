# StreamArr Pro

**Self-hosted media server for Live TV, Movies & Series with Xtream Codes & M3U8 support**

Generate dynamic playlists for Live TV, Movies and TV Series using Xtream Codes compatible API. Stream content via Real-Debrid, Torrentio, Comet, MediaFusion and direct sources. Perfect for IPTV apps like TiviMate, iMPlayer, IPTV Smarters Pro, XCIPTV, Kodi and more.

[![Download ZIP](https://img.shields.io/badge/Download%20ZIP-latest-blue?style=for-the-badge&logo=github)](https://github.com/Zerr0-C00L/StreamArr/archive/refs/heads/main.zip)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue?style=for-the-badge&logo=docker)](https://github.com/Zerr0-C00L/StreamArr)
[![Ko-fi](https://www.ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/zeroq)

---

## 🎬 Features

- **Movies & Series** - Add content from TMDB with full metadata, posters, and descriptions
- **Movie Collections** - Auto-detect and complete collections (add one MCU movie, get them all)
- **Live TV** - 700+ channels with EPG (DrewLive, DaddyLive, PlutoTV)
- **Multi-Provider Streaming** - Real-Debrid, Torrentio, Comet, MediaFusion with automatic fallback
- **MDBList Sync** - Auto-sync your watchlists and custom lists
- **Modern Web UI** - Beautiful dashboard, library browser, and settings
- **Xtream Codes API** - Full compatibility with all IPTV apps
- **Background Services** - Auto-sync collections, search streams, update EPG

---

## 🚀 Installation

### Option 1: Docker (Recommended)

The easiest way to get started. Requires [Docker](https://docs.docker.com/get-docker/) and Docker Compose.

```bash
# Clone the repository
git clone https://github.com/Zerr0-C00L/StreamArr.git
cd StreamArr

# Start with Docker Compose
docker compose up -d

# View logs
docker compose logs -f streamarr
```

**That's it!** Access the Web UI at http://localhost:8080

The database is automatically set up and migrations are applied.

### Option 2: Manual Installation

<details>
<summary>Click to expand manual installation steps</summary>

#### Prerequisites
- Go 1.21+ ([download](https://golang.org/dl/))
- PostgreSQL 14+ ([download](https://www.postgresql.org/download/))
- Node.js 18+ ([download](https://nodejs.org/)) - only needed if building UI from source

#### Step 1: Clone and Configure

```bash
# Clone the repository
git clone https://github.com/Zerr0-C00L/StreamArr.git
cd StreamArr

# Copy environment file
cp .env.example .env

# Edit .env with your settings (at minimum, set DATABASE_URL)
nano .env
```

#### Step 2: Set Up PostgreSQL

```bash
# Create database and user
sudo -u postgres psql

# In psql:
CREATE USER streamarr WITH PASSWORD 'streamarr';
CREATE DATABASE streamarr OWNER streamarr;
\q
```

#### Step 3: Run Migrations

```bash
# Apply all migrations
psql postgres://streamarr:streamarr@localhost:5432/streamarr -f migrations/001_initial_schema.up.sql
psql postgres://streamarr:streamarr@localhost:5432/streamarr -f migrations/002_add_settings.up.sql
psql postgres://streamarr:streamarr@localhost:5432/streamarr -f migrations/003_add_users.up.sql
psql postgres://streamarr:streamarr@localhost:5432/streamarr -f migrations/004_add_collections.up.sql
psql postgres://streamarr:streamarr@localhost:5432/streamarr -f migrations/005_add_collection_checked.up.sql
```

#### Step 4: Build and Run

```bash
# Build the server
go build -o bin/server cmd/server/main.go

# Build the UI (optional - pre-built UI is included)
cd streamarr-pro-ui && npm install && npm run build && cd ..

# Start the server
./bin/server
```

Access the Web UI at http://localhost:8080

</details>

### Option 3: VPS Deployment (Hetzner, DigitalOcean, etc.)

<details>
<summary>Click to expand VPS installation guide</summary>

#### Step 1: Create a VPS

Create a VPS with any provider (Hetzner, DigitalOcean, Vultr, Linode, etc.):
- **OS**: Ubuntu 22.04 or 24.04 LTS
- **RAM**: 2GB minimum (4GB recommended)
- **Storage**: 20GB minimum

#### Step 2: Connect and Update

```bash
# SSH into your server
ssh root@YOUR-SERVER-IP

# Update system
apt update && apt upgrade -y
```

#### Step 3: Install Docker

```bash
# Install Docker
curl -fsSL https://get.docker.com | sh

# Start Docker
systemctl enable docker
systemctl start docker
```

#### Step 4: Install StreamArr

```bash
# Clone repository
git clone https://github.com/Zerr0-C00L/StreamArr.git
cd StreamArr

# Start with Docker Compose
docker compose up -d

# Check if running
docker compose ps
```

#### Step 5: Configure Firewall

```bash
# Allow port 8080
ufw allow 8080/tcp
ufw enable
```

#### Step 6: Access StreamArr

Open in your browser: `http://YOUR-SERVER-IP:8080`

#### Optional: Setup Domain with HTTPS (Recommended)

```bash
# Install Caddy (automatic HTTPS)
apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt update && apt install caddy

# Create Caddyfile
cat > /etc/caddy/Caddyfile << 'EOF'
your-domain.com {
    reverse_proxy localhost:8080
}
EOF

# Restart Caddy
systemctl restart caddy
```

Now access via `https://your-domain.com`

#### Useful Commands

```bash
# View logs
docker compose logs -f

# Restart services
docker compose restart

# Update to latest version
cd StreamArr
git pull
docker compose build
docker compose up -d
```

</details>

---

## ⚙️ First-Time Setup

After installation, open http://localhost:8080 and go to **Settings**:

| Setting | Description | Required |
|---------|-------------|----------|
| **TMDB API Key** | For movie/series metadata. [Get free key](https://developer.themoviedb.org/docs/getting-started) | ✅ Yes |
| **Real-Debrid Key** | Premium cached torrents. [Get key](https://real-debrid.com/apitoken) | Optional |
| **MDBList API Key** | Watchlist sync. [Get key](https://mdblist.com/preferences/) | Optional |

---

## 📱 IPTV App Setup

### Xtream Codes Login (Recommended)

Most IPTV apps support Xtream Codes. Use these settings:

| Field | Value |
|-------|-------|
| **Server URL** | `http://YOUR-IP:8080` |
| **Username** | `user` (or anything) |
| **Password** | `pass` (or anything) |

### M3U Playlist URL

For apps without Xtream support:
```
http://YOUR-IP:8080/get.php?username=user&password=pass&type=m3u_plus&output=ts
```

### Supported Apps

| App | Platform | Xtream Support |
|-----|----------|----------------|
| TiviMate | Android/TV | ✅ Yes |
| iMPlayer | iOS/Apple TV | ✅ Yes |
| IPTV Smarters Pro | All | ✅ Yes |
| XCIPTV | Android | ✅ Yes |
| OTT Navigator | Android | ✅ Yes |
| Kodi (PVR IPTV) | All | M3U only |
| VLC | All | M3U only |

---

## 🔧 Adding Content

### Add Movies/Series

1. Go to **Movies** or **Series** in the sidebar
2. Click **+ Add** button
3. Search for content by name
4. Click the result to add it to your library

### Auto-Add Collections

When you add a movie that's part of a collection (like Marvel, Star Wars, etc.), StreamArr can automatically add all other movies in that collection.

Enable in **Settings → Quality → Auto-add Collections**

### MDBList Sync

1. Get your API key from [mdblist.com/preferences](https://mdblist.com/preferences/)
2. Go to **Settings → MDBList** and enter your key
3. Your lists will sync automatically every 6 hours

---

## 🐳 Docker Commands

```bash
# Start services
docker compose up -d

# View logs
docker compose logs -f

# Stop services
docker compose down

# Rebuild after updates
git pull
docker compose build
docker compose up -d

# Reset database (WARNING: deletes all data)
docker compose down -v
docker compose up -d
```

---

## 🔄 Updating

### Docker
```bash
cd StreamArr
git pull
docker compose build
docker compose up -d
```

### Manual
```bash
cd StreamArr
git pull
go build -o bin/server cmd/server/main.go
# Restart the server
```

---

## 🛠️ Troubleshooting

<details>
<summary><b>Server won't start</b></summary>

1. Check PostgreSQL is running: `sudo systemctl status postgresql`
2. Verify DATABASE_URL is correct in .env
3. Check logs: `docker compose logs streamarr`
</details>

<details>
<summary><b>No streams found</b></summary>

1. Ensure at least one provider is enabled in Settings → Providers
2. For Real-Debrid, verify your API key is valid
3. Check if the content has available torrents
</details>

<details>
<summary><b>IPTV app can't connect</b></summary>

1. Use your server's IP address, not `localhost`
2. Ensure port 8080 is open in firewall
3. Try the full URL format: `http://IP:8080`
</details>

<details>
<summary><b>Live TV channels not loading</b></summary>

1. Go to Settings → Services
2. Manually trigger "Channel Refresh" service
3. Wait for EPG Update to complete
</details>

---

## 📊 API Endpoints

### Xtream Codes API
- `GET /player_api.php` - Xtream Codes compatible API
- `GET /get.php` - Playlist generation

### REST API (v1)
- `GET /api/v1/movies` - List movies
- `GET /api/v1/series` - List series  
- `GET /api/v1/channels` - List channels
- `GET /api/v1/health` - Health check

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

---

## 📝 License

MIT License - see [LICENSE.md](LICENSE.md) for details.

---

## ☕ Support

If you find this project useful, consider supporting:

[![Ko-fi](https://www.ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/zeroq)

---

## ⚠️ Disclaimer

This software is for personal use only. Users are responsible for ensuring they have the right to access any content they stream.
