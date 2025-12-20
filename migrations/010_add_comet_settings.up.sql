-- Add Comet provider settings
ALTER TABLE settings ADD COLUMN IF NOT EXISTS comet_enabled BOOLEAN DEFAULT true;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS comet_indexers TEXT DEFAULT 'bitorrent,therarbg,yts,eztv,thepiratebay';
