-- Add series and episode support to media_streams table
-- Allow stream caching for both movies and series

-- Add series/episode columns to media_streams
ALTER TABLE media_streams ADD COLUMN IF NOT EXISTS series_id BIGINT REFERENCES library_series(id) ON DELETE CASCADE;
ALTER TABLE media_streams ADD COLUMN IF NOT EXISTS season INTEGER;
ALTER TABLE media_streams ADD COLUMN IF NOT EXISTS episode INTEGER;

-- Drop the old movie-only unique constraint
ALTER TABLE media_streams DROP CONSTRAINT IF EXISTS unique_movie_stream;

-- Add new constraints: unique stream per movie OR per episode
CREATE UNIQUE INDEX IF NOT EXISTS unique_movie_stream_idx ON media_streams (movie_id) WHERE movie_id IS NOT NULL AND series_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS unique_episode_stream_idx ON media_streams (series_id, season, episode) WHERE series_id IS NOT NULL;

-- Add check constraint: must have either movie_id or series_id (not both, not neither)
ALTER TABLE media_streams ADD CONSTRAINT check_media_type CHECK (
    (movie_id IS NOT NULL AND series_id IS NULL) OR 
    (movie_id IS NULL AND series_id IS NOT NULL)
);

-- Add index for series lookup
CREATE INDEX IF NOT EXISTS idx_streams_series_id ON media_streams (series_id) WHERE series_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_streams_series_season ON media_streams (series_id, season) WHERE series_id IS NOT NULL;

-- Record migration
INSERT INTO schema_migrations (version, applied_at)
VALUES (14, NOW())
ON CONFLICT (version) DO NOTHING;
