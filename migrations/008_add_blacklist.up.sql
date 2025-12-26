-- Migration: 008_add_blacklist.up.sql
-- Add blacklist table to prevent re-importing removed items

-- Create blacklist table if it doesn't exist
CREATE TABLE IF NOT EXISTS blacklist (
    id BIGSERIAL PRIMARY KEY,
    tmdb_id INTEGER NOT NULL,
    item_type TEXT NOT NULL CHECK (item_type IN ('movie', 'series')),
    title TEXT NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tmdb_id, item_type)
);

-- Create index on tmdb_id for fast lookups during import
CREATE INDEX IF NOT EXISTS idx_blacklist_tmdb_id ON blacklist(tmdb_id, item_type);
CREATE INDEX IF NOT EXISTS idx_blacklist_created_at ON blacklist(created_at DESC);
