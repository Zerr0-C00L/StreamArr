-- Migration: 004_add_collections.down.sql
-- Remove movie collections support

DROP TRIGGER IF EXISTS trig_update_collection_timestamp ON collections;
DROP FUNCTION IF EXISTS update_collection_timestamp();

ALTER TABLE library_movies DROP COLUMN IF EXISTS collection_id;

DROP TABLE IF EXISTS collections;
