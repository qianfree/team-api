-- +goose Up
-- 将 tnt_users.email 由 NOT NULL 改为可空。
--
-- 背景：普通成员的邮箱在系统中基本用不上——成员只能用 RAM 账号（username@tenant_code）
-- 登录（admin 邮箱登录只认 role=owner 的主用户），且成员不能凭邮箱自助重置密码，
-- 邮箱仅用于邮件通知与成员列表展示。因此将邀请加入、管理员创建、CSV 导入三条
-- 创建路径统一为「邮箱选填」，与 CreateMember / member_import 既有的选填行为对齐。
--
-- 顺带修复既有隐患：原 email 为 NOT NULL + UNIQUE(tenant_id, email)，不填邮箱时
-- 代码只能写入空字符串 ""，而空字符串是确定值，导致同一租户下仅能存在一个空邮箱
-- 成员，第二个会撞唯一约束报「邮箱已存在」。改为可空后，PostgreSQL 唯一索引允许
-- 多个 NULL，多个无邮箱成员不再冲突。
--
-- 注意：列的可空性变更不需要重新 gf gen dao——DO 中 Email 字段本就声明为 any 类型，
-- 业务代码在 email 为空时传 nil 即可写入 SQL NULL。

ALTER TABLE tnt_users ALTER COLUMN email DROP NOT NULL;

-- 将已存在的空字符串 email 归一为 NULL，避免旧空串数据继续占用唯一约束位。
UPDATE tnt_users SET email = NULL WHERE email = '';

-- +goose Down
-- 恢复 NOT NULL 前需保证无 NULL email（否则 SET NOT NULL 会失败），回退为空字符串。
UPDATE tnt_users SET email = '' WHERE email IS NULL;
ALTER TABLE tnt_users ALTER COLUMN email SET NOT NULL;
