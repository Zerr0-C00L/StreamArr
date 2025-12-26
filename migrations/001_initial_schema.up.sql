-- Migration: 001_initial_schema.up.sql
-- Create initial database schema for StreamArr

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS pg_trgm;  -- For fuzzy text search
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Movies table
CREATE TABLE library_movies (
    id BIGSERIAL PRIMARY KEY,
    tmdb_id INTEGER NOT NULL UNIQUE,
    imdb_id TEXT,
    title TEXT NOT NULL,
    year INTEGER,
    monitored BOOLEAN DEFAULT true,
    clean_title TEXT NOT NULL,
    
    -- Full-text search
    title_vector tsvector,
    
    -- Metadata (JSONB for flexibility)
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Status
    added_at TIMESTAMPTZ DEFAULT NOW(),
    last_checked TIMESTAMPTZ,
    available BOOLEAN DEFAULT false,
    
    -- Quality preferences
    preferred_quality TEXT DEFAULT '1080p',
    min_quality TEXT DEFAULT '720p'
);

-- Indexes for movies (critical for 200k rows)
CREATE INDEX idx_movies_monitored ON library_movies(monitored) WHERE monitored = true;
CREATE INDEX idx_movies_available ON library_movies(available) WHERE available = true;
CREATE INDEX idx_movies_title_vector ON library_movies USING GIN(title_vector);
CREATE INDEX idx_movies_metadata ON library_movies USING GIN(metadata);
CREATE INDEX idx_movies_added_at ON library_movies(added_at DESC);
CREATE INDEX idx_movies_clean_title ON library_movies(clean_title);
CREATE INDEX idx_movies_year ON library_movies(year);

-- Trigger to update title_vector
CREATE OR REPLACE FUNCTION update_movie_title_vector() RETURNS trigger AS $$
BEGIN
  NEW.title_vector := to_tsvector('english', NEW.title);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trig_update_movie_title_vector
  BEFORE INSERT OR UPDATE OF title ON library_movies
  FOR EACH ROW
  EXECUTE FUNCTION update_movie_title_vector();

-- Series table
CREATE TABLE library_series (
    id BIGSERIAL PRIMARY KEY,
    tmdb_id INTEGER NOT NULL UNIQUE,
    imdb_id TEXT,
    title TEXT NOT NULL,
    year INTEGER,
    monitored BOOLEAN DEFAULT true,
    clean_title TEXT NOT NULL,
    title_vector tsvector,
    metadata JSONB NOT NULL DEFAULT '{}',
    added_at TIMESTAMPTZ DEFAULT NOW(),
    last_checked TIMESTAMPTZ,
    preferred_quality TEXT DEFAULT '1080p'
);

CREATE INDEX idx_series_monitored ON library_series(monitored) WHERE monitored = true;
CREATE INDEX idx_series_title_vector ON library_series USING GIN(title_vector);
CREATE INDEX idx_series_added_at ON library_series(added_at DESC);

CREATE TRIGGER trig_update_series_title_vector
  BEFORE INSERT OR UPDATE OF title ON library_series
  FOR EACH ROW
  EXECUTE FUNCTION update_movie_title_vector();

-- Episodes table (partitioned by series_id for performance)
CREATE TABLE library_episodes (
    id BIGSERIAL,
    series_id BIGINT NOT NULL REFERENCES library_series(id) ON DELETE CASCADE,
    season_number INTEGER NOT NULL,
    episode_number INTEGER NOT NULL,
    title TEXT,
    overview TEXT,
    air_date DATE,
    monitored BOOLEAN DEFAULT true,
    available BOOLEAN DEFAULT false,
    last_checked TIMESTAMPTZ,
    still_path TEXT,
    PRIMARY KEY (id, series_id)
) PARTITION BY HASH (series_id);

-- Create 16 partitions for episodes
CREATE TABLE library_episodes_0 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 0);
CREATE TABLE library_episodes_1 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 1);
CREATE TABLE library_episodes_2 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 2);
CREATE TABLE library_episodes_3 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 3);
CREATE TABLE library_episodes_4 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 4);
CREATE TABLE library_episodes_5 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 5);
CREATE TABLE library_episodes_6 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 6);
CREATE TABLE library_episodes_7 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 7);
CREATE TABLE library_episodes_8 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 8);
CREATE TABLE library_episodes_9 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 9);
CREATE TABLE library_episodes_10 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 10);
CREATE TABLE library_episodes_11 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 11);
CREATE TABLE library_episodes_12 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 12);
CREATE TABLE library_episodes_13 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 13);
CREATE TABLE library_episodes_14 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 14);
CREATE TABLE library_episodes_15 PARTITION OF library_episodes FOR VALUES WITH (MODULUS 16, REMAINDER 15);

CREATE INDEX idx_episodes_series_season ON library_episodes(series_id, season_number);
CREATE INDEX idx_episodes_air_date ON library_episodes(air_date) WHERE air_date IS NOT NULL;
CREATE INDEX idx_episodes_monitored ON library_episodes(monitored) WHERE monitored = true;
CREATE UNIQUE INDEX idx_episodes_unique ON library_episodes(series_id, season_number, episode_number);

-- Available streams table (expect 1M+ rows)
CREATE TABLE available_streams (
    id BIGSERIAL PRIMARY KEY,
    movie_id BIGINT REFERENCES library_movies(id) ON DELETE CASCADE,
    episode_id BIGINT,
    quality TEXT NOT NULL,
    size_bytes BIGINT,
    release_name TEXT,
    release_group TEXT,
    torrent_hash TEXT,
    rd_link TEXT,
    cached_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    score INTEGER DEFAULT 0,
    
    CONSTRAINT check_content CHECK (
        (movie_id IS NOT NULL AND episode_id IS NULL) OR
        (movie_id IS NULL AND episode_id IS NOT NULL)
    )
);

CREATE INDEX idx_streams_movie ON available_streams(movie_id, quality, score DESC) WHERE movie_id IS NOT NULL;
CREATE INDEX idx_streams_episode ON available_streams(episode_id, quality, score DESC) WHERE episode_id IS NOT NULL;
CREATE INDEX idx_streams_expires ON available_streams(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_streams_hash ON available_streams(torrent_hash);

-- Watch history
CREATE TABLE watch_history (
    id BIGSERIAL PRIMARY KEY,
    movie_id BIGINT REFERENCES library_movies(id) ON DELETE CASCADE,
    episode_id BIGINT,
    stream_id BIGINT REFERENCES available_streams(id) ON DELETE SET NULL,
    watched_at TIMESTAMPTZ DEFAULT NOW(),
    progress_seconds INTEGER DEFAULT 0,
    completed BOOLEAN DEFAULT false
);

CREATE INDEX idx_watch_history_movie ON watch_history(movie_id, watched_at DESC);
CREATE INDEX idx_watch_history_episode ON watch_history(episode_id, watched_at DESC);
CREATE INDEX idx_watch_history_recent ON watch_history(watched_at DESC) WHERE completed = false;

-- Activity log
CREATE TABLE activity_log (
    id BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    content_type TEXT NOT NULL,
    content_id BIGINT NOT NULL,
    message TEXT,
    data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_activity_created ON activity_log(created_at DESC);
CREATE INDEX idx_activity_type ON activity_log(event_type, created_at DESC);

-- Quality profiles
CREATE TABLE quality_profiles (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    qualities JSONB NOT NULL,
    cutoff TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert default quality profiles
INSERT INTO quality_profiles (name, qualities, cutoff) VALUES
('HD', '["2160p", "1080p", "720p"]', '1080p'),
('SD', '["720p", "480p"]', '720p'),
('Any', '["2160p", "1080p", "720p", "480p"]', '480p'),
('4K Preferred', '["2160p", "1080p"]', '2160p');

-- Notification settings
CREATE TABLE notification_settings (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL UNIQUE,
    enabled BOOLEAN DEFAULT false,
    config JSONB NOT NULL,
    events JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
