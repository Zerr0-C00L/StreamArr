-- Remove Comet provider settings
DELETE FROM settings WHERE key IN (
    'comet_enabled',
    'comet_indexers', 
    'comet_only_show_cached',
    'comet_max_results',
    'comet_sort_by',
    'comet_excluded_qualities',
    'comet_priority_languages',
    'comet_max_size'
);
