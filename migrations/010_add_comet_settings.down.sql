-- Remove Comet provider settings
DELETE FROM settings WHERE key IN ('comet_enabled', 'comet_indexers', 'comet_only_show_cached');
