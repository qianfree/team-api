-- +goose Up
ALTER TABLE opn_apps ADD COLUMN IF NOT EXISTS encrypted_secret TEXT;
COMMENT ON COLUMN opn_apps.encrypted_secret IS 'AES-256 encrypted App Secret for HMAC verification';

UPDATE api_keys
SET key_type = 'project'
WHERE project_id IS NOT NULL
  AND key_type = 'personal';

-- +goose Down
ALTER TABLE opn_apps DROP COLUMN IF EXISTS encrypted_secret;
