# Balkan VOD Feature

## Overview
Automatically import domestic movies and series from Ex-Yugoslavia region (Serbian, Croatian, Bosnian content) from the [Balkan-On-Demand](https://github.com/Zerr0-C00L/Balkan-On-Demand) GitHub repository.

## Features
- ✅ Automatic import of Ex-Yu content from GitHub repos
- ✅ Category filtering (domestic content only)
- ✅ Auto-sync for new content
- ✅ Configurable sync intervals
- ✅ Enable/disable via Settings UI
- ✅ Background worker for automatic updates

## Content Included
The feature imports content from these categories:
- **EX YU FILMOVI** - Ex-Yu movies
- **EX YU SERIJE** - Ex-Yu TV series
- **KLIK PREMIJERA** - Premiere releases
- **KLASICI / FILMSKI KLASICI** - Classic films
- Popular series: Bolji Zivot, Bela Ladja, Policajac Sa Petlovog Brda, Slatke Muke

## Content Excluded
Foreign content is automatically filtered out:
- STRANI HD FILMOVI (Foreign movies)
- Crtani Filmovi (Cartoons)
- DOKUMENTARCI (Documentaries)
- General international series

## How to Enable

### 1. Via Web UI (Recommended)
1. Navigate to **Settings** → **Live TV** tab
2. Scroll to **"🇧🇦 Balkan/Ex-Yu VOD from GitHub"** section
3. Toggle the switch to enable
4. Configure sync options:
   - **Auto-sync new content**: Automatically import new content
   - **Sync interval**: How often to check for new content (default: 24 hours)
   - **Category Selection**: Click "Select Categories" to choose specific categories (default: all domestic)
5. Click **Save Settings**

### Category Selection
- Click **"Select Categories"** button to open category browser
- See all available categories with item counts
- Use **"Select All"** or **"Deselect All"** for bulk operations
- Leave all unchecked to import all domestic categories
- Selected categories persist across syncs

### 2. Via API
```bash
curl -X POST http://localhost:8080/api/v1/admin/settings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "balkan_vod_enabled": true,
    "balkan_vod_auto_sync": true,
    "balkan_vod_sync_interval_hours": 24
  }'
```

## Settings

### `balkan_vod_enabled` (boolean)
- **Default**: `false`
- **Description**: Master switch to enable/disable Balkan VOD import

### `balkan_vod_auto_sync` (boolean)
- **Default**: `true`
- **Description**: Automatically sync new content on schedule
- **Note**: Only active when `balkan_vod_enabled` is true

### `balkan_vod_sync_interval_hours` (integer)
- **Default**: `24`
- **Options**: 6, 12, 24, 48, 72, 168 (weekly)
- **Description**: Hours between automatic sync checks

### `balkan_vod_selected_categories` (array of strings)
- **Default**: `[]` (empty = all domestic categories)
- **Description**: List of category names to import
- **Example**: `["EX YU FILMOVI", "KLIK PREMIJERA", "KLASICI"]`
- **Note**: Only imports from domestic categories list for safety

## Content Sources
The feature fetches content from:
- **Primary**: [Balkan-On-Demand Full Backup](https://raw.githubusercontent.com/Zerr0-C00L/Balkan-On-Demand/main/data/baubau-content-full-backup.json)
  - 18,123+ movies
  - 37+ series
  - Includes streams, posters, descriptions

## How It Works

1. **Background Worker**: Runs every N hours (configurable)
2. **Fetches Data**: Downloads latest content from GitHub
3. **Filters Content**: Applies domestic category filters
4. **Imports**: Adds new content to your library
5. **Deduplication**: Skips content already in library
6. **Metadata**: Stores streams in movie/series metadata

## Content Storage
Imported content includes:
- **Title, Year, Poster, Background**
- **Description, Genres, Runtime**
- **Streams**: Direct video URLs stored in metadata
- **Category**: Original category from source
- **Import Timestamp**: When content was added

## Metadata Example
```json
{
  "source": "balkan_vod",
  "imported_at": "2025-12-18T12:00:00Z",
  "category": "EX YU FILMOVI",
  "balkan_vod_streams": [
    {
      "name": "Balkan VOD",
      "url": "https://...",
      "quality": "1080p"
    }
  ]
}
```

## Manual Import
To trigger an immediate import without waiting for the sync interval:
1. Restart the worker: `./scripts/stop.sh && ./scripts/start.sh`
2. Worker will run sync on startup if enabled
3. Check logs: `tail -f logs/worker.log`

## Logs
Look for these log entries:
```
🇧🇦 Balkan VOD Sync Worker: Starting (interval: 24h0m0s)
🇧🇦 Balkan VOD Sync: Starting import from GitHub repos...
[BalkanVOD] Fetched 18123 movies and 37 series
[BalkanVOD] Import complete: 645 imported, 17478 skipped, 0 failed
✅ Balkan VOD Sync complete
```

## Troubleshooting

### Content not importing?
- Check settings: `balkan_vod_enabled` must be `true`
- Check logs: `tail -f logs/worker.log`
- Verify GitHub repo is accessible
- Check worker is running: `ps aux | grep worker`

### Duplicate content?
- Import uses TMDB ID for deduplication
- If TMDB ID is missing (0), content may be duplicated
- Future enhancement: Use stream URL for deduplication

### Want to remove imported content?
Content is stored in your library. To remove:
1. Disable the feature: Uncheck "Import Balkan/Ex-Yu VOD"
2. Manually remove content from Library page (filter by category or source)

## Future Enhancements
- [ ] TMDB ID matching via title search
- [ ] Stream URL deduplication
- [ ] DomaciFlix integration (second GitHub source)
- [ ] Manual "Import Now" button in UI
- [ ] Progress tracking during import
- [ ] Import statistics on Dashboard

## Files Modified
- `internal/models/settings.go` - Added settings fields + selected_categories
- `internal/settings/manager.go` - Added default values + selected_categories
- `internal/services/balkan_vod_importer.go` - Import logic with category filtering
- `internal/api/handlers.go` - Added PreviewBalkanCategories endpoint
- `internal/api/routes.go` - Added category preview route
- `internal/services/scheduler.go` - Added service constant
- `cmd/worker/main.go` - Added background worker
- `streamarr-pro-ui/src/pages/Settings.tsx` - Moved to Live TV tab, added category UI

## Credits
Content sourced from:
- [Balkan-On-Demand](https://github.com/Zerr0-C00L/Balkan-On-Demand) by Zerr0-C00L
- Community-maintained database of Ex-Yu movies and series
