# StreamArr Pro

<p align="center">
  <img src="streamarr-pro-ui/public/logo.png" alt="StreamArr Pro" width="120">
</p>

<p align="center">
  <b>Self-hosted media server with Netflix-style UI for Movies, Series & Live TV</b>
</p>

<p align="center">
  Integrates with user-configured providers (e.g., Stremio-compatible addons) and optional premium caching services for personal streaming.<br>
  Exposes Xtream-compatible endpoints for use with IPTV apps.
</p>

<p align="center">
  <a href="https://github.com/Zerr0-C00L/StreamArr/archive/refs/heads/main.zip"><img src="https://img.shields.io/badge/Download-ZIP-blue?style=for-the-badge&logo=github" alt="Download"></a>
  <a href="#-docker-installation"><img src="https://img.shields.io/badge/Docker-Ready-blue?style=for-the-badge&logo=docker" alt="Docker"></a>
  <a href="https://ko-fi.com/zeroq"><img src="https://img.shields.io/badge/Support-Ko--fi-ff5f5f?style=for-the-badge&logo=ko-fi" alt="Ko-fi"></a>
</p>

---

## 📸 Screenshots

<img width="1470" height="956" alt="Screenshot 2025-12-15 at 7 14 02 am" src="https://github.com/user-attachments/assets/1f38d243-c68c-4b89-9a4d-dd3e63517ab4" />
<img width="1470" height="956" alt="Screenshot 2025-12-15 at 7 13 45 am" src="https://github.com/user-attachments/assets/a80802ca-4c8e-49d7-a463-71cc5968b817" />
<img width="1470" height="956" alt="Screenshot 2025-12-15 at 7 14 14 am" src="https://github.com/user-attachments/assets/ad8219d7-da4c-404c-8f76-43e9b5b19937" />
<img width="1470" height="956" alt="Screenshot 2025-12-15 at 7 14 43 am" src="https://github.com/user-attachments/assets/b06b7f24-fb5a-4940-ba1b-0d76c469749d" />
<img width="1470" height="956" alt="Screenshot 2025-12-15 at 7 15 02 am" src="https://github.com/user-attachments/assets/c1ec1d3d-ff43-43c5-b719-0bd28e804cb2" />
<img width="1470" height="956" alt="Screenshot 2025-12-15 at 7 15 13 am" src="https://github.com/user-attachments/assets/737a5b6f-20d7-47d3-a55e-3f751fa0af0f" />

---

## ✨ Features

### 🎬 Content Management
| Feature | Description |
|---------|-------------|
| **Movies & Series** | Add any movie or TV series from TMDB with full metadata, posters, backdrops, and descriptions |
| **Episode Browser** | Browse all seasons and episodes with air dates, thumbnails, and per-episode stream fetching |
| **Collections** | Auto-detect movie collections (Marvel, Star Wars, etc.) and optionally add all movies in the collection |
| **MDBList Sync** | Auto-import from your MDBList watchlists, trending lists, and custom lists |
| **Smart Search** | Search TMDB for content, see what's trending, and discover popular movies/series |

### 📺 Streaming
| Feature | Description |
|---------|-------------|
| **Stremio-Compatible Providers** | Supports user-added providers (e.g., Torrentio, Comet, MediaFusion) |
| **Premium Caching** | Optional integration with caching services to accelerate access to user-available sources |
| **Multi-Provider** | Configure multiple addons with automatic fallback if one fails |
| **Stream Selection** | See all available streams with quality, size, codec, seeders, and actual filename |
| **Quality Filters** | Filter by resolution (4K, 1080p, 720p), exclude CAM/TS, filter languages |

### 📡 Live TV
| Feature | Description |
|---------|-------------|
| **Live TV via M3U** | Use your own M3U playlists and IPTV sources |
| **EPG Support** | Electronic Program Guide when available from configured sources |
| **Custom M3U** | Add your own M3U playlists and IPTV sources |
| **Source & Category Filters** | Enable/disable sources and categories (Sports, News, Movies, Entertainment, Kids, etc.) |

### 📱 IPTV Compatibility
| Feature | Description |
|---------|-------------|
| **Xtream-Compatible API** | Works with TiviMate, iMPlayer, IPTV Smarters, XCIPTV, OTT Navigator |
| **M3U Playlist** | Standard M3U8 playlist for VLC, Kodi, and any M3U-compatible player |
| **Apple TV** | Works with Chillio, iPlayTV, and other Apple TV IPTV apps |
| **VOD Support** | Movies and Series appear as VOD content in IPTV apps |
<br/>
Note: Use only with content you are legally entitled to access. Examples below use placeholder credentials; configure your own secure values.

### 🎨 Web Interface
| Feature | Description |
|---------|-------------|
| **Netflix-Style UI** | Modern dark theme with horizontal scrolling rows, hover cards, and smooth animations |
| **Detail Modals** | Click any movie/series to see full details, seasons, episodes, and available streams |
| **Responsive Design** | Works on desktop, tablet, and mobile browsers |
| **Dashboard** | Quick stats, recently added content, and upcoming releases |
| **Calendar** | See upcoming movie releases and episode air dates |

### ⚙️ Advanced Features
| Feature | Description |
|---------|-------------|
| **Background Services** | Auto-sync MDBList, refresh channels, update EPG on schedule |
| **Release Filters** | Exclude specific release groups, languages, or qualities |
| **Stream Sorting** | Sort by quality, size, seeders, or cached status |
| **User Accounts** | Multi-user support with admin controls |
| **API Access** | Full REST API for integration with other tools |

---

## 🚀 Quick Start

### Docker Installation (Recommended)

```bash
# Clone the repository
git clone https://github.com/Zerr0-C00L/StreamArr_Pro.git
cd StreamArr_Pro

# Start with Docker Compose
docker compose up -d

# View logs (optional)
docker compose logs -f streamarr_pro
```

**Done!** Open http://localhost:8080 in your browser.

---

## ⚙️ Initial Setup

After installation, go to **Settings** (gear icon) and configure:

### 1. API Keys Tab
| Setting | Description | Required |
|---------|-------------|----------|
| **TMDB API Key** | For movie/series metadata. [Get free key →](https://developer.themoviedb.org/docs/getting-started) | ✅ Required |
| **MDBList API Key** | For watchlist sync. [Get key →](https://mdblist.com/preferences/) | Optional |

### 2. Debrid Services
| Service | Description |
|---------|-------------|
| **Premium Caching (e.g., Real-Debrid)** | Optional cached resolution. [Get API key →](https://real-debrid.com/apitoken) |
| **Premiumize** | Alternative debrid service |
| **TorBox** | Coming soon |

### 3. Stremio Addons
Add your Stremio-compatible provider URLs. Examples:

| Addon | URL | Notes |
|-------|-----|-------|
| **Comet** | `https://comet.elfhosted.com` | Fast, good cache detection |
| **Torrentio** | `https://torrentio.strem.fun` | Most sources, highly configurable |
| **MediaFusion** | `https://mediafusion.elfhosted.com` | Good alternative |

> **Tip:** You can add multiple addons. StreamArr will try each one in order until it finds streams.

---

## 📱 IPTV App Setup

### Xtream Codes Login (All IPTV Apps)

| Field | Value |
|-------|-------|
| **Server URL** | `http://YOUR-IP:8080` |
| **Username** | Set in Settings → Xtream (default: `user`) |
| **Password** | Set in Settings → Xtream (default: `pass`) |

### M3U Playlist URL (VLC, Kodi)
```
http://YOUR-IP:8080/get.php?username=user&password=pass&type=m3u_plus&output=ts
```

### Tested Apps

| App | Platform | Status |
|-----|----------|--------|
| **TiviMate** | Android TV | ✅ Excellent |
| **iMPlayer** | iOS / Apple TV | ✅ Excellent |
| **Chillio** | Apple TV | ✅ Excellent |
| **IPTV Smarters Pro** | All platforms | ✅ Works |
| **XCIPTV** | Android | ✅ Works |
| **OTT Navigator** | Android | ✅ Works |
| **Kodi (PVR IPTV)** | All platforms | ✅ M3U only |
| **VLC** | All platforms | ✅ M3U only |

---

## 🎯 How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                        StreamArr Pro                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────┐    ┌──────────────┐    ┌────────────────────┐    │
│  │  Web UI  │───▶│   REST API   │───▶│  Stream Providers  │    │
│  │ (React)  │    │    (Go)      │    │  (Comet/Torrentio) │    │
│  └──────────┘    └──────────────┘    └────────────────────┘    │
│       │                │                       │                │
│       │                ▼                       ▼                │
│       │         ┌──────────────┐    ┌────────────────────┐     │
│       │         │  PostgreSQL  │    │    Real-Debrid     │     │
│       │         │   Database   │    │  (Cached Streams)  │     │
│       │         └──────────────┘    └────────────────────┘     │
│       │                                        │                │
│       ▼                                        ▼                │
│  ┌──────────────────────────────────────────────────────┐      │
│  │              Xtream Codes API                         │      │
│  │     /player_api.php  •  /movie/  •  /series/         │      │
│  └──────────────────────────────────────────────────────┘      │
│                            │                                    │
│                            ▼                                    │
│  ┌──────────────────────────────────────────────────────┐      │
│  │                    IPTV Apps                          │      │
│  │  TiviMate • iMPlayer • Chillio • IPTV Smarters       │      │
│  └──────────────────────────────────────────────────────┘      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Flow Explanation

1. **Add Content** → Search TMDB, add movies/series to your library
2. **Metadata Stored** → Titles, posters, descriptions saved to PostgreSQL
3. **IPTV App Request** → App requests movie/episode via Xtream API
4. **Fetch Streams** → StreamArr queries user-configured providers for available sources
5. **Premium Caching** → Optional cached resolution via premium services where permitted
6. **Playback** → Direct high-speed stream URL returned to IPTV app

---

## 🖥️ Web UI Guide

### Navigation
| Page | Description |
|------|-------------|
| **Home** | Dashboard with stats, recent content, upcoming releases |
| **Library** | Browse all movies and series with Netflix-style rows |
| **Live TV** | Browse and filter live TV channels |
| **Discover** | Search TMDB, see trending content, add to library |
| **Settings** | Configure API keys, addons, quality, filters |

### Library Features
- **Hero Banner** - Featured content with backdrop image
- **Content Rows** - Horizontal scrolling rows for Movies and Series
- **Hover Cards** - Preview poster with play button on hover
- **Detail Modal** - Click to see full info, seasons, episodes, streams
- **Stream Selection** - See all available streams with:
  - Quality badge (4K/1080p/720p)
  - Source (Comet/Torrentio)
  - Cached status (⚡ instant playback)
  - File size in GB
  - Codec (HEVC/H.264)
  - Full filename (click to expand)

### Adding Content
1. Click **Discover** in the navigation
2. Search for a movie or series
3. Click the **+** button to add to library
4. Content appears in Library immediately

---

## 🔧 Advanced Configuration

### Quality Settings
- **Max Resolution** - Limit to 4K, 1080p, or 720p
- **Max File Size** - Skip files over a certain size
- **Quality Variants** - Show all qualities or just the best

### Release Filters
Filter out unwanted releases by:
- **Release Groups** - Exclude specific groups (e.g., YIFY)
- **Languages** - Exclude foreign language releases
- **Qualities** - Exclude CAM, TS, SCR, HDTS

### Stream Sorting
Sort streams by:
- Quality (highest first)
- Size (largest first)
- Seeds (most seeded first)
- Cached status (cached first)

---

## 📡 Live TV Sources

### Built-in Sources
| Source | Channels | Region |
|--------|----------|--------|
| PlutoTV | varies | User-configured |
| Custom M3U | varies | User-configured |

### Adding Custom Sources
1. Go to **Settings → Live TV**
2. Add M3U URL under "M3U Sources"
3. Click Save, then refresh channels

### EPG (Program Guide)
EPG is automatically loaded from multiple sources (XMLTV, Pluto TV, and custom URLs from your M3U sources). Manual refresh available in **Settings → Services**.

---

## 🐳 Docker Commands

```bash
# Start
docker compose up -d

# Stop
docker compose down

# View logs
docker compose logs -f streamarr

# Restart
docker compose restart streamarr

# Update to latest
git pull
docker compose build --no-cache
docker compose up -d

# Reset database (WARNING: deletes all data)
docker compose down -v
docker compose up -d
```

---

## 🔄 Updating

```bash
cd StreamArr
git pull
docker compose build
docker compose up -d
```

---

## 🛠️ Troubleshooting

<details>
<summary><b>No streams found for movies/series</b></summary>

1. **Check Addons** - Go to Settings → Addons, ensure at least one is enabled
2. **Check Real-Debrid** - Verify API key in Settings → API Keys
3. **Check Addon URLs** - Make sure URLs are correct (no trailing slash)
4. **Try Different Content** - Some obscure content may not have torrents

</details>

<details>
<summary><b>IPTV app won't connect</b></summary>

1. **Use IP Address** - Use your server's IP, not `localhost`
2. **Check Port** - Ensure port 8080 is accessible
3. **Check Credentials** - Use credentials from Settings → Xtream
4. **Full URL** - Some apps need `http://` prefix

</details>

<details>
<summary><b>Streams buffer or won't play</b></summary>

1. **Check Real-Debrid** - Verify your subscription is active
2. **Cached Streams** - Prefer streams with ⚡ Cached badge
3. **Lower Quality** - Try 1080p instead of 4K
4. **Different Stream** - Try a different source

</details>

<details>
<summary><b>Episodes not showing streams</b></summary>

1. **IMDB ID** - Ensure the series has an IMDB ID (check metadata)
2. **Air Date** - Episode must have aired
3. **Refresh** - Try removing and re-adding the series

</details>

<details>
<summary><b>Live TV not working</b></summary>

1. **Refresh Channels** - Settings → Services → Channel Refresh
2. **Enable Sources** - Settings → Live TV → Enable sources
3. **Wait for EPG** - EPG can take a few minutes to load

</details>

---

## 📊 API Reference

### Xtream Codes Endpoints
| Endpoint | Description |
|----------|-------------|
| `GET /player_api.php` | Main Xtream Codes API |
| `GET /get.php` | Playlist generation |
| `GET /movie/{user}/{pass}/{id}.mp4` | Movie stream |
| `GET /series/{user}/{pass}/{id}.mp4` | Episode stream |
| `GET /live/{user}/{pass}/{id}.m3u8` | Live channel |
| `GET /xmltv.php` | EPG data |
<br/>
For personal use. Endpoints surface content only from user-configured, lawful sources.

### REST API (v1)
| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/movies` | List all movies |
| `GET /api/v1/movies/{id}/streams` | Get movie streams |
| `GET /api/v1/series` | List all series |
| `GET /api/v1/series/{id}/episodes` | Get series episodes |
| `GET /api/v1/stream/series/{imdb}:{s}:{e}` | Get episode streams |
| `GET /api/v1/channels` | List live channels |
| `GET /api/v1/health` | Health check |

---

## 🤝 Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Submit a Pull Request

---

## 📝 License

MIT License - see [LICENSE.md](LICENSE.md)

---

## ☕ Support

If StreamArr Pro is useful to you, consider supporting development:

<a href="https://ko-fi.com/zeroq"><img src="https://www.ko-fi.com/img/githubbutton_sm.svg" alt="Support on Ko-fi"></a>

---

## 🛡️ Responsible Use

StreamArr Pro is a self-hosted media organizer. It does not host, index, or distribute any media. You are responsible for ensuring that any configured sources and services comply with local laws and their terms of service. Use only with content you are legally entitled to access.

---

## 🔗 Content & Providers

StreamArr Pro does not include or endorse any particular provider. References to third-party addons or services (e.g., Stremio-compatible providers, caching services) are examples. Configuration and usage are at the user’s discretion and risk.

---

## ⚠️ Disclaimer

This software is for personal, lawful use only. The project does not host, provide, index, or distribute any copyrighted content, nor does it encourage infringement. Users must ensure compliance with applicable laws, platform policies, and terms of service for any third-party services they configure.
