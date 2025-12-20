-- Add Comet provider settings
INSERT INTO settings (key, value, type) VALUES ('comet_enabled', 'true', 'bool') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_indexers', 'bitorrent,therarbg,yts,eztv,thepiratebay', 'string') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_only_show_cached', 'true', 'bool') ON CONFLICT (key) DO NOTHING;
