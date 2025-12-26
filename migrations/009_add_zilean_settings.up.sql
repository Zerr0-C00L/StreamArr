-- Add Zilean DMM integration settings
ALTER TABLE settings ADD COLUMN IF NOT EXISTS zilean_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS zilean_url TEXT DEFAULT 'http://localhost:8181';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS zilean_api_key TEXT DEFAULT '';
