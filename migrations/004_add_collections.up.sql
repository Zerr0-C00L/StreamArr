-- Migration: 004_add_collections.up.sql
-- Add movie collections support

-- Collections table (TMDB collections like "The Dark Knight Trilogy", "MCU", etc.)
CREATE TABLE collections (
    id BIGSERIAL PRIMARY KEY,
    tmdb_id INTEGER NOT NULL UNIQUE,
    name TEXT NOT NULL,
    overview TEXT,
    poster_path TEXT,
    backdrop_path TEXT,
    total_movies INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_collections_tmdb_id ON collections(tmdb_id);
CREATE INDEX IF NOT EXISTS idx_collections_name ON collections(name);

-- Add collection_id to movies table
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'library_movies' AND column_name = 'collection_id'
    ) THEN
        ALTER TABLE library_movies ADD COLUMN collection_id BIGINT REFERENCES collections(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_movies_collection ON library_movies(collection_id) WHERE collection_id IS NOT NULL;

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_collection_timestamp() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trig_update_collection_timestamp
  BEFORE UPDATE ON collections
  FOR EACH ROW
  EXECUTE FUNCTION update_collection_timestamp();
