-- Add Stream Checker (Phase 1 Cache) settings
INSERT INTO settings (key, value, type) VALUES
    ('cache_check_interval_minutes', '60', 'int'),
    ('cache_check_batch_size', '50', 'int'),
    ('cache_auto_upgrade', 'true', 'bool'),
    ('cache_min_upgrade_points', '15', 'int'),
    ('cache_max_upgrade_size_gb', '20', 'int')
ON CONFLICT (key) DO NOTHING;
