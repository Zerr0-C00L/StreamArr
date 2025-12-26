-- Add media_streams table for stream caching (Phase 1: No AI needed)
-- This enables instant playback by caching one debrid-cached stream per media item

CREATE TABLE IF NOT EXISTS media_streams (
    id              SERIAL PRIMARY KEY,
    media_type      VARCHAR(10) NOT NULL,  -- 'movie' or 'series'
    media_id        INTEGER NOT NULL,      -- References library_movies.id or library_series.id
    stream_url      TEXT NOT NULL,
    stream_hash     VARCHAR(64),        -- For duplicate detection
    quality_score   INTEGER,            -- 0-100 score (algorithmic, no AI)
    resolution      VARCHAR(20),        -- 4K, 1080p, 720p, SD
    hdr_type        VARCHAR(20),        -- DV, HDR10+, HDR10, SDR
    audio_format    VARCHAR(50),        -- Atmos, TrueHD, DTS-HD, AC3
    source_type     VARCHAR(20),        -- Remux, BluRay, WEB-DL, WEBRip
    file_size_gb    DECIMAL(10,2),
    codec           VARCHAR(20),        -- x265, x264, AV1
    indexer         VARCHAR(50),        -- Which indexer found it
    cached_at       TIMESTAMP DEFAULT NOW(),
    last_checked    TIMESTAMP DEFAULT NOW(),
    check_count     INTEGER DEFAULT 0,
    is_available    BOOLEAN DEFAULT true,
    upgrade_available BOOLEAN DEFAULT false,
    next_check_at   TIMESTAMP DEFAULT NOW() + INTERVAL '7 days',
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW(),
    UNIQUE(media_type, media_id)  -- One stream per media item
);

-- Index for scheduled checks (background worker queries this)
CREATE INDEX IF NOT EXISTS idx_streams_next_check ON media_streams (next_check_at) 
WHERE is_available = true;

-- Index for media lookup (fast retrieval on playback)
CREATE INDEX IF NOT EXISTS idx_streams_media_type_id ON media_streams (media_type, media_id);

-- Index for quality score (finding upgrade candidates)
CREATE INDEX IF NOT EXISTS idx_streams_quality ON media_streams (quality_score);

-- Index for hash lookup (duplicate detection)
CREATE INDEX IF NOT EXISTS idx_streams_hash ON media_streams (stream_hash) 
WHERE stream_hash IS NOT NULL;
