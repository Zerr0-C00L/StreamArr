-- Add advanced Comet provider settings
INSERT INTO settings (key, value, type) VALUES ('comet_max_results', '5', 'int') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_sort_by', 'quality', 'string') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_excluded_qualities', '', 'string') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_priority_languages', '', 'string') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value, type) VALUES ('comet_max_size', '', 'string') ON CONFLICT (key) DO NOTHING;
