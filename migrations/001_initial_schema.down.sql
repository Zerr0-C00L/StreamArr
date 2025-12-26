-- Migration: 001_initial_schema.down.sql
-- Rollback initial schema

DROP TABLE IF EXISTS notification_settings CASCADE;
DROP TABLE IF EXISTS quality_profiles CASCADE;
DROP TABLE IF EXISTS activity_log CASCADE;
DROP TABLE IF EXISTS watch_history CASCADE;
DROP TABLE IF EXISTS available_streams CASCADE;
DROP TABLE IF EXISTS library_episodes CASCADE;
DROP TABLE IF EXISTS library_series CASCADE;
DROP TABLE IF EXISTS library_movies CASCADE;

DROP FUNCTION IF EXISTS update_movie_title_vector CASCADE;
