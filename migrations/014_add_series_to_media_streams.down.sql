-- Rollback series/episode support from media_streams

-- Remove constraints
ALTER TABLE media_streams DROP CONSTRAINT IF EXISTS check_media_type;
DROP INDEX IF EXISTS unique_episode_stream_idx;
DROP INDEX IF EXISTS unique_movie_stream_idx;
DROP INDEX IF EXISTS idx_streams_series_id;
DROP INDEX IF EXISTS idx_streams_series_season;

-- Remove series columns
ALTER TABLE media_streams DROP COLUMN IF EXISTS series_id;
ALTER TABLE media_streams DROP COLUMN IF EXISTS season;
ALTER TABLE media_streams DROP COLUMN IF EXISTS episode;

-- Restore original movie-only constraint
ALTER TABLE media_streams ADD CONSTRAINT unique_movie_stream UNIQUE(movie_id);

-- Remove migration record
DELETE FROM schema_migrations WHERE version = 14;
