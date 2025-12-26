-- Restore Comet provider settings (if needed to rollback)
INSERT INTO settings (key, value, type) VALUES ('comet_enabled', 'false', 'bool') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_indexers', 'bitorrent,therarbg,yts,eztv,thepiratebay', 'string') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_only_show_cached', 'true', 'bool') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_max_results', '0', 'int') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_sort_by', 'qualitySize', 'string') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_excluded_qualities', '', 'string') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_priority_languages', '', 'string') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_max_size', '0', 'int') ON CONFLICT (key) DO NOTHING;
