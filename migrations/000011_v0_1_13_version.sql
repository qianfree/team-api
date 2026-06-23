-- +goose Up

-- P1-1: 管理员暴力破解防护 — 新增失败计数和锁定字段
ALTER TABLE sys_admin_users
    ADD COLUMN IF NOT EXISTS failed_attempts INT DEFAULT 0 NOT NULL,
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;

COMMENT ON COLUMN sys_admin_users.failed_attempts IS '连续登录失败次数（成功登录后归零）';
COMMENT ON COLUMN sys_admin_users.locked_until IS '锁定截止时间（连续5次失败后锁定30分钟）';

-- P1-3: TOTP 密钥列宽度不足 — AES-GCM 加密 + base64 编码后约 80 字符，原 VARCHAR(64) 溢出
ALTER TABLE sys_admin_users ALTER COLUMN totp_secret TYPE VARCHAR(255);
ALTER TABLE tnt_users ALTER COLUMN totp_secret TYPE VARCHAR(255);

-- 开放平台应用密钥加密存储（HMAC 校验用）
ALTER TABLE opn_apps ADD COLUMN IF NOT EXISTS encrypted_secret TEXT;
COMMENT ON COLUMN opn_apps.encrypted_secret IS 'AES-256 encrypted App Secret for HMAC verification';

-- 修正项目 Key 类型：绑定项目的 Key 应为 project 类型而非 personal
UPDATE api_keys
SET key_type = 'project'
WHERE project_id IS NOT NULL
  AND key_type = 'personal';

-- 团队功能开关：false=个人模式（默认），true=已激活团队
ALTER TABLE tnt_tenants
  ADD COLUMN IF NOT EXISTS team_enabled BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN tnt_tenants.team_enabled IS '团队功能是否启用：false=个人模式（默认），true=已激活团队（成员/RAM/邀请/额度）';

-- 存量租户均通过旧注册流程设置过自定义 code，视为团队已启用
UPDATE tnt_tenants SET team_enabled = TRUE WHERE team_enabled = FALSE;

-- +goose Down

-- 团队功能开关（team_enabled 的存量数据回填不可逆，仅回滚结构）
ALTER TABLE tnt_tenants DROP COLUMN IF EXISTS team_enabled;

-- 开放平台应用密钥（api_keys.key_type 的数据修正是不可逆的数据变更，仅回滚结构）
ALTER TABLE opn_apps DROP COLUMN IF EXISTS encrypted_secret;

-- TOTP 密钥列宽度还原
ALTER TABLE sys_admin_users ALTER COLUMN totp_secret TYPE VARCHAR(64);
ALTER TABLE tnt_users ALTER COLUMN totp_secret TYPE VARCHAR(64);

-- 管理员暴力破解防护字段
ALTER TABLE sys_admin_users
    DROP COLUMN IF EXISTS failed_attempts,
    DROP COLUMN IF EXISTS locked_until;
