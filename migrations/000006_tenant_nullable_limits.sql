-- +goose Up
-- 租户 max_members 和 max_concurrency 改为可 NULL
-- NULL 表示跟随等级配置（tnt_tenant_level_configs），非 NULL 表示自定义覆盖

ALTER TABLE tnt_tenants ALTER COLUMN max_members DROP NOT NULL;
ALTER TABLE tnt_tenants ALTER COLUMN max_members SET DEFAULT NULL;
ALTER TABLE tnt_tenants ALTER COLUMN max_concurrency DROP NOT NULL;
ALTER TABLE tnt_tenants ALTER COLUMN max_concurrency SET DEFAULT NULL;

COMMENT ON COLUMN tnt_tenants.max_members IS '最大成员数上限（NULL表示跟随等级配置）';
COMMENT ON COLUMN tnt_tenants.max_concurrency IS '租户总并发上限（NULL表示跟随等级配置，0表示不限制）';

-- 将现有记录的值保留（已有具体数值的不做变更，保持向后兼容）

-- +goose Down
-- 恢复为 NOT NULL，将 NULL 值设为等级对应配置值，无等级配置的回退到默认值

UPDATE tnt_tenants t
SET max_members = COALESCE(
    (SELECT lc.max_members FROM tnt_tenant_level_configs lc WHERE lc.level = t.level),
    10
)
WHERE t.max_members IS NULL;

UPDATE tnt_tenants t
SET max_concurrency = COALESCE(
    (SELECT lc.max_concurrency FROM tnt_tenant_level_configs lc WHERE lc.level = t.level),
    0
)
WHERE t.max_concurrency IS NULL;

ALTER TABLE tnt_tenants ALTER COLUMN max_members SET DEFAULT 10;
ALTER TABLE tnt_tenants ALTER COLUMN max_members SET NOT NULL;
ALTER TABLE tnt_tenants ALTER COLUMN max_concurrency SET DEFAULT 0;
ALTER TABLE tnt_tenants ALTER COLUMN max_concurrency SET NOT NULL;

COMMENT ON COLUMN tnt_tenants.max_members IS '最大成员数上限';
COMMENT ON COLUMN tnt_tenants.max_concurrency IS '租户总并发上限（0表示不限制）';
