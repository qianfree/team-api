-- +goose Up
-- Sync is_public flag from code registry to DB rows.
-- Settings like turnstile_enabled/turnstile_site_key were marked IsPublic in the
-- registry but the DB rows still had is_public=false, causing GetPublicOptions
-- (WHERE is_public=true) to skip them and return stale defaults to the tenant frontend.

UPDATE sys_options SET is_public = true, updated_at = NOW()
WHERE key IN (
    'site_name',
    'site_description',
    'register_enabled',
    'register_email_verification',
    'maintenance_message',
    'maintenance_duration',
    'api_maintenance_enabled',
    'oauth_auto_register',
    'oauth_github_enabled',
    'oauth_google_enabled',
    'turnstile_enabled',
    'turnstile_site_key',
    'agreement_enabled'
) AND is_public = false;

-- +goose Down
-- No-op: reverting is_public to false would break public settings again,
-- so we leave the corrected values in place.
SELECT 1;
