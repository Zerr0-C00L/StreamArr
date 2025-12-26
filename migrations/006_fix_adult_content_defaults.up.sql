-- Fix adult content settings to ensure they default to false
-- This migration updates existing settings to explicitly set these values to false
-- if they were previously unset or incorrectly set

-- Update app_settings to ensure adult content flags are explicitly false
UPDATE settings 
SET value = jsonb_set(
    jsonb_set(
        value::jsonb,
        '{include_adult_vod}', 
        'false'::jsonb,
        true
    ),
    '{import_adult_vod_from_github}', 
    'false'::jsonb,
    true
)::text
WHERE key = 'app_settings'
  AND (
    (value::jsonb->'include_adult_vod') IS NULL 
    OR (value::jsonb->'include_adult_vod')::text = 'true'
    OR (value::jsonb->'import_adult_vod_from_github') IS NULL
    OR (value::jsonb->'import_adult_vod_from_github')::text = 'true'
  );
