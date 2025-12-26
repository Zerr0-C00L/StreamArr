-- Add settings table
CREATE TABLE IF NOT EXISTS settings (
    id SERIAL PRIMARY KEY,
    key VARCHAR(255) UNIQUE NOT NULL,
    value TEXT NOT NULL,
    type VARCHAR(50) NOT NULL DEFAULT 'string',
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Add updated_at column if it doesn't exist (for existing installations)
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'settings' AND column_name = 'updated_at'
    ) THEN
        ALTER TABLE settings ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT NOW();
    END IF;
END $$;

-- Create index if not exists
CREATE INDEX IF NOT EXISTS idx_settings_key ON settings(key);

-- Insert default settings
INSERT INTO settings (key, value, type) VALUES
    ('tmdb_api_key', '', 'string'),
    ('realdebrid_token', '', 'string'),
    ('premiumize_api_key', '', 'string'),
    ('mdblist_api_key', '', 'string'),
    ('user_create_playlist', 'false', 'bool'),
    ('total_pages', '5', 'int'),
    ('language', 'en-US', 'string'),
    ('movies_origin_country', '', 'string'),
    ('series_origin_country', '', 'string'),
    ('m3u8_limit', '0', 'int'),
    ('include_live_tv', 'false', 'bool'),
    ('include_adult_vod', 'false', 'bool'),
    ('max_resolution', '1080', 'int'),
    ('max_file_size', '50000', 'int'),
    ('enable_quality_variants', 'false', 'bool'),
    ('show_full_stream_name', 'false', 'bool'),
    ('use_realdebrid', 'false', 'bool'),
    ('use_premiumize', 'false', 'bool'),
    ('mediafusion_enabled', 'false', 'bool'),
    ('torrentio_providers', 'yts,eztv,rarbg,1337x,thepiratebay,kickasstorrents,torrentgalaxy', 'string'),
    ('include_popular_movies', 'false', 'bool'),
    ('include_top_rated_movies', 'false', 'bool'),
    ('include_now_playing', 'false', 'bool'),
    ('include_upcoming', 'false', 'bool'),
    ('include_latest_releases_movies', 'false', 'bool'),
    ('include_collections', 'false', 'bool'),
    ('include_popular_series', 'false', 'bool'),
    ('include_top_rated_series', 'false', 'bool'),
    ('include_airing_today', 'false', 'bool'),
    ('include_on_the_air', 'false', 'bool'),
    ('include_latest_releases_series', 'false', 'bool'),
    ('enable_release_filters', 'false', 'bool'),
    ('excluded_release_groups', 'TVHUB|FILM|Ultradox|RUSSIAN|HINDI', 'string'),
    ('excluded_languages', 'RUSSIAN|HINDI|GERMAN|FRENCH|ITALIAN|SPANISH|PORTUGUESE', 'string'),
    ('excluded_qualities', 'REMUX|HDR|CAM|HDCAM|TS', 'string'),
    ('user_set_host', '', 'string'),
    ('expiration_hours', '3', 'int'),
    ('auto_cache_interval_hours', '6', 'int'),
    ('timeout', '20', 'int'),
    ('use_github_for_cache', 'false', 'bool'),
    ('debug', 'false', 'bool')
ON CONFLICT (key) DO NOTHING;

