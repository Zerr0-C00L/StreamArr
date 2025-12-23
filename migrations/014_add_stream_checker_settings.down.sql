-- Remove Stream Checker settings
DELETE FROM settings WHERE key IN (
    'cache_check_interval_minutes',
    'cache_check_batch_size',
    'cache_auto_upgrade',
    'cache_min_upgrade_points',
    'cache_max_upgrade_size_gb'
);
