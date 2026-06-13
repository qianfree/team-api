-- +goose Up
-- Sync is_public flag from code registry to DB rows.
-- Settings like turnstile_enabled/turnstile_site_key were marked IsPublic in the
-- registry but the DB rows still had is_public=false, causing GetPublicOptions
-- (WHERE is_public=true) to skip them and return stale defaults to the tenant frontend.
--
-- 变更记录：
--   2026-06-13 追加 'maintenance_mode'。租户前端 MaintenanceBanner.vue 的
--     `v-if="settings.maintenance_mode"` 依赖公开配置返回维护模式开关，但
--     registry 此前未标 IsPublic、DB 行也未置 public，导致维护横幅即使管理员
--     开启维护模式也永不显示。已在 settings_registry.go 同步标记 IsPublic=true。
--     若该行在 DB 中尚不存在（管理员从未保存过维护配置），公开接口会通过
--     registry 默认值（false）兜底返回，不影响功能。

UPDATE sys_options SET is_public = true, updated_at = NOW()
WHERE key IN (
    'site_name',
    'site_description',
    'register_enabled',
    'register_email_verification',
    'maintenance_mode',
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
