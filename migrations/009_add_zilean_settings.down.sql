-- Remove Zilean DMM integration settings
ALTER TABLE settings DROP COLUMN IF EXISTS zilean_api_key;
ALTER TABLE settings DROP COLUMN IF EXISTS zilean_url;
ALTER TABLE settings DROP COLUMN IF EXISTS zilean_enabled;
