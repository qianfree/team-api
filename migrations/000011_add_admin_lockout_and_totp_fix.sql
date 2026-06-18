-- +goose Up

-- P1-1: 管理员暴力破解防护 — 新增失败计数和锁定字段
ALTER TABLE sys_admin_users
    ADD COLUMN failed_attempts INT DEFAULT 0 NOT NULL,
    ADD COLUMN locked_until TIMESTAMPTZ;

COMMENT ON COLUMN sys_admin_users.failed_attempts IS '连续登录失败次数（成功登录后归零）';
COMMENT ON COLUMN sys_admin_users.locked_until IS '锁定截止时间（连续5次失败后锁定30分钟）';

-- P1-3: TOTP 密钥列宽度不足 — AES-GCM 加密 + base64 编码后约 80 字符，原 VARCHAR(64) 溢出
ALTER TABLE sys_admin_users ALTER COLUMN totp_secret TYPE VARCHAR(255);
ALTER TABLE tnt_users ALTER COLUMN totp_secret TYPE VARCHAR(255);

-- +goose Down

ALTER TABLE sys_admin_users
    DROP COLUMN IF EXISTS failed_attempts,
    DROP COLUMN IF EXISTS locked_until;

ALTER TABLE sys_admin_users ALTER COLUMN totp_secret TYPE VARCHAR(64);
ALTER TABLE tnt_users ALTER COLUMN totp_secret TYPE VARCHAR(64);
