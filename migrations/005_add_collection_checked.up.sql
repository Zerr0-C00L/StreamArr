-- Migration: 005_add_collection_checked.up.sql
-- Add collection_checked flag to track which movies have been scanned for collection membership

ALTER TABLE library_movies ADD COLUMN IF NOT EXISTS collection_checked BOOLEAN DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_movies_collection_checked ON library_movies(collection_checked) WHERE collection_checked = FALSE;
