-- Migration: 005_add_collection_checked.down.sql
-- Remove collection_checked flag

DROP INDEX IF EXISTS idx_movies_collection_checked;
ALTER TABLE library_movies DROP COLUMN IF EXISTS collection_checked;
