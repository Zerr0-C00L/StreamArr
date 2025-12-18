#!/bin/bash

# Add GitHub repos as import sources in StreamArr
# These repos will be available as VOD import sources

cat << 'EOF'

═══════════════════════════════════════════════════════════
  Add GitHub Repos to StreamArr VOD Import
═══════════════════════════════════════════════════════════

Your GitHub repos can be imported directly! Here's how:

1. In StreamArr UI → Settings → VOD Import Sources

2. Add Xtream/M3U Sources:

   Option A: Use the extracted M3U files (easiest)
   ─────────────────────────────────────────────
   - Name: "Balkan Domestic Content"
   - Type: M3U URL
   - URL: http://localhost:8080/static/balkan_vod_repos.m3u
   
   (First copy the M3U to static folder):
   mkdir -p static
   cp channels/balkan_vod_repos.m3u static/

   Option B: Create a GitHub M3U proxy endpoint
   ──────────────────────────────────────────────
   Add this endpoint to serve content directly from GitHub:
   
   GET /api/v1/vod/github/balkan-on-demand.m3u
   GET /api/v1/vod/github/domaciflix.m3u

   Option C: Direct Xtream API Import
   ───────────────────────────────────
   Create an Xtream API wrapper that:
   - Fetches: baubau-content-full-backup.json
   - Transforms to Xtream API format
   - Serves via: /player_api.php endpoints

═══════════════════════════════════════════════════════════

RECOMMENDED: Option A (Simplest)
────────────────────────────────────────────────────────────

1. Run the extraction script:
   python3 extract-from-repos.py

2. Copy M3U to static folder:
   mkdir -p static
   cp channels/balkan_vod_repos.m3u static/

3. In StreamArr UI:
   - Settings → VOD Import
   - Add Source: "Balkan Repos"
   - Type: M3U File
   - Path: static/balkan_vod_repos.m3u
   - Categories: [Select Movies/Series]
   - Click Import

This will import all domestic content with full TMDB metadata!

EOF
