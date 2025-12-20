-- Remove Comet provider settings
ALTER TABLE settings DROP COLUMN IF EXISTS comet_enabled;
ALTER TABLE settings DROP COLUMN IF EXISTS comet_indexers;
