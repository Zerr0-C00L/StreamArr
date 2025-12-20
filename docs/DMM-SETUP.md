# Debrid Media Manager (DMM) Integration

StreamArr Pro now integrates with Debrid Media Manager (DMM) to provide pre-scraped torrent results from a continuously updated database.

## Architecture

```
┌─────────────┐      ┌─────────────┐      ┌──────────────┐
│ StreamArr   │─────▶│ DMM API     │─────▶│ DMM Database │
│  (Frontend) │      │ (Container) │      │ (PostgreSQL) │
└─────────────┘      └─────────────┘      └──────────────┘
                            │
                            ▼
                     ┌──────────────┐
                     │ DMM Scraper  │ (Background)
                     │ - Torrentio  │
                     │ - YTS        │
                     │ - 1337x      │
                     │ - etc...     │
                     └──────────────┘
```

## What is DMM?

DMM (Debrid Media Manager) is a torrent search and management system that:
- **Pre-scrapes** torrents from multiple sources (Torrentio, 1337x, YTS, etc.)
- Stores results in a **database** for instant retrieval
- Provides an **API** to query torrents by IMDb ID
- **Continuously updates** the database with new releases

## Why Use DMM?

✅ **Bypasses Torrentio Blocks** - If Torrentio is blocked in your region, DMM scrapes it beforehand  
✅ **Instant Results** - No waiting for live searches, results come from database  
✅ **Multiple Sources** - Aggregates from Torrentio, YTS, 1337x, and more  
✅ **Always Updated** - Background scraper keeps database fresh  
✅ **Real-Debrid Ready** - Only returns cached torrents compatible with RD

## How It Works

1. **User clicks "Watch"** on a movie/series in StreamArr
2. **StreamArr queries DMM API**: `GET /api/torrents/movie?imdbId=tt1234567`
3. **DMM returns pre-scraped results** from its database
4. **StreamArr displays streams** to user
5. **User selects stream** → StreamArr fetches from Real-Debrid

## Setup

### Option 1: Using Docker Compose (Recommended)

DMM is already configured in `docker-compose.yml`:

```bash
# Start DMM and its database
docker compose up -d dmm dmm-db

# Check DMM logs
docker compose logs -f dmm

# Verify DMM is running
curl http://localhost:8081/healthz
```

**DMM Configuration:**
- **API Port**: `8081` (mapped to container port `8080`)
- **Database**: PostgreSQL 17.2 (separate container)
- **Health Check**: `http://localhost:8081/healthz`
- **Container Network**: `streamarr-pro_default`

### Option 2: Using Existing DMM Instance

If you already have DMM running elsewhere, update the provider:

```go
// In internal/providers/dmm_direct.go
func NewDMMDirectProvider(rdAPIKey string) *DMMDirectProvider {
    return &DMMDirectProvider{
        DMMURL: "http://your-dmm-instance:8080", // Change this
        // ... rest of config
    }
}
```

## DMM API Endpoints

StreamArr uses these DMM endpoints:

| Endpoint | Purpose | Example |
|----------|---------|---------|
| `/api/torrents/movie` | Get movie torrents | `GET /api/torrents/movie?imdbId=tt0111161` |
| `/api/torrents/tv` | Get TV season torrents | `GET /api/torrents/tv?imdbId=tt0903747&seasonNum=1` |
| `/healthz` | Health check | `GET /healthz` |

## DMM Response Format

```json
{
  "results": [
    {
      "title": "The Shawshank Redemption 1994 1080p BluRay x264",
      "fileSize": 1433.5,  // Size in MB
      "hash": "a0b1c2d3e4f5..."
    }
  ]
}
```

## DMM Database Population

DMM needs time to scrape and populate its database:

1. **First 24-48 hours**: DMM scrapes popular content (top 1000 movies/shows)
2. **On-demand scraping**: When you request an IMDb ID not in database:
   - Returns `204` with `status: requested` header
   - Queues IMDb ID for scraping
   - Try again in 1-2 minutes
3. **Continuous updates**: DMM re-scrapes content every 7 days

## Checking DMM Status

```bash
# Check if DMM containers are running
docker compose ps dmm dmm-db

# View DMM logs (see scraping activity)
docker compose logs -f dmm

# Check database size
docker compose exec dmm-db psql -U dmm -c "SELECT COUNT(*) FROM torrents;"

# View latest scraped content
docker compose exec dmm-db psql -U dmm -c "SELECT imdb_id, title, updated_at FROM media ORDER BY updated_at DESC LIMIT 10;"
```

## Troubleshooting

### "DMM is still scraping this content"
- **Cause**: Content not yet in DMM database
- **Solution**: Wait 1-2 minutes and try again, or check DMM logs

### "DMM API returned status 403"
- **Cause**: Authentication token validation failed
- **Solution**: Check DMM configuration, token generation is automatic

### "Connection refused to dmm:8080"
- **Cause**: DMM container not running or network issue
- **Solution**: 
  ```bash
  docker compose up -d dmm dmm-db
  docker network ls | grep streamarr
  ```

### No results returned
- **Cause**: DMM database empty (fresh install)
- **Solution**: Wait 24-48 hours for initial scraping, or manually trigger:
  ```bash
  # Trigger scrape for specific IMDb ID (requires DMM API key)
  curl -X POST http://localhost:8081/api/scrape \
    -H "Content-Type: application/json" \
    -d '{"imdbId": "tt0111161"}'
  ```

## Performance

- **Cache Duration**: 10 minutes per IMDb ID
- **API Response Time**: ~50-200ms (from database)
- **Database Size**: ~5-10GB for 10,000 movies/shows
- **Memory Usage**: ~512MB-1GB for DMM container

## Alternative: Torrentio Direct (if DMM is down)

If DMM is unavailable, StreamArr can still query Torrentio directly by modifying the provider to use the old implementation. This is useful for debugging or if you don't need the DMM database.

## Resources

- **DMM GitHub**: https://github.com/debridmediamanager/debrid-media-manager
- **Zilean (DMM Backend)**: https://github.com/ipromknight/zilean
- **StreamArr DMM Provider**: `internal/providers/dmm_direct.go`

## Configuration Summary

```yaml
# docker-compose.yml
dmm:
  image: ipromknight/zilean:latest
  ports:
    - "8081:8080"
  environment:
    - Zilean__Database__ConnectionString=Host=dmm-db;Database=dmm;...
    - Zilean__Dmm__EnableEndpoint=true
    - Zilean__Dmm__MinimumScoreMatch=0.85
```

```go
// StreamArr DMM Provider
DMMURL: "http://dmm:8080"  // Internal Docker network
Cache: 10 minutes per IMDb ID
Timeout: 25 seconds per request
```
