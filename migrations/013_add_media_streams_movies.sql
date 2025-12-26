-- Add media_streams table for stream caching (Phase 1: No AI needed)
-- This enables instant playback by caching one debrid-cached stream per media item
-- Modified to work with existing library_movies table

CREATE TABLE IF NOT EXISTS media_streams (
    id              SERIAL PRIMARY KEY,
    movie_id        BIGINT REFERENCES library_movies(id) ON DELETE CASCADE,
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
    CONSTRAINT unique_movie_stream UNIQUE(movie_id)
);

-- Index for scheduled checks (background worker queries this)
CREATE INDEX IF NOT EXISTS idx_streams_next_check ON media_streams (next_check_at) 
WHERE is_available = true;

-- Index for movie lookup (fast retrieval on playback)
CREATE INDEX IF NOT EXISTS idx_streams_movie_id ON media_streams (movie_id);

-- Index for quality score (finding upgrade candidates)
CREATE INDEX IF NOT EXISTS idx_streams_quality ON media_streams (quality_score);

-- Index for hash lookup (duplicate detection)
CREATE INDEX IF NOT EXISTS idx_streams_hash ON media_streams (stream_hash) 
WHERE stream_hash IS NOT NULL;

-- Record migration
INSERT INTO schema_migrations (version, applied_at)
VALUES (13, NOW())
ON CONFLICT (version) DO NOTHING;
