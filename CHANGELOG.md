# Changelog

## v1.1.0 — 2025-12-17

Highlights
- Fast VOD import (no TMDB): Rapidly ingest IPTV VOD using parsed title/year, synthetic stable IDs, and direct source metadata.
- Adult VOD import from GitHub: New import path and service to add 18+ VOD content governed by settings.
- Stream selection split: VOD-imported items only show VOD links; discovery/MDBList items show only addon/torrent streams.
- Robust routing for VOD import: Added fallbacks for `/iptv-vod/import` (with/without `/v1`, trailing slash).
- TMDB client fix: Correct URL composition preventing double base prepend and 204s.
- Collection guard for VOD: Skip "Add Entire Collection" behaviors for VOD-only items/collections.

Backend
- Added `IPTVVODFastImport` setting; importer branches to TMDB-free flow when enabled.
- Implemented fast import helpers: `importMovieBasic` / `importSeriesBasic` with negative synthetic IDs and `iptv_vod_sources` metadata.
- Introduced `ImportAdultMoviesFromGitHub(ctx, movieStore)` service; handler now delegates to this service.
- Updated streams API: `GetMovieStreams` prefers direct VOD links for `iptv_vod` items, and filters providers for non-VOD.
- Added helpers: `buildIPTVVODStreams`, `isIPTVVODMovie`, `collectionHasNonIPTVVOD`.
- Adjusted add/sync logic to bypass collection auto-add for VOD scenarios.
- Fixed TMDB request builder to safely compose base URL and query params.
- Routing: registered `/api/v1/iptv-vod/import` plus fallbacks at `/api/iptv-vod/import` and trailing-slash variant.
- Adult VOD: `ImportAdultVOD` refactored to service; added `/adult-vod/import` and `/adult-vod/stats` endpoints.

Frontend
- Settings UI expanded to include Fast VOD import toggle (where applicable).

Stability & Ops
- Build and container restart verified; health endpoint stable.
- Version bumped to 1.1.0 (ldflags still supported for CI builds).

Notes
- Optional future work: background enrichment for fast-imported items, scheduler wiring for adult import, and UI toggle for GitHub adult VOD.
