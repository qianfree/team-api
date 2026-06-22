-- +goose Up
ALTER TABLE tnt_tenants
  ADD COLUMN IF NOT EXISTS team_enabled BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN tnt_tenants.team_enabled IS '团队功能是否启用：false=个人模式（默认），true=已激活团队（成员/RAM/邀请/额度）';

-- 存量租户均通过旧注册流程设置过自定义 code，视为团队已启用
UPDATE tnt_tenants SET team_enabled = TRUE WHERE team_enabled = FALSE;

-- +goose Down
ALTER TABLE tnt_tenants DROP COLUMN IF EXISTS team_enabled;
