-- +goose Up
-- 新增 member:manage 权限点：管理后台的成员禁用/启用/重置密码/解锁。
--
-- 此前这四个写接口挂在只读的 member:view 上（internal/middleware/rbac.go），
-- 于是「技术支持」这类只读定位的角色可以重置任意租户成员的密码 —— 改密等于接管该成员
-- 账号，进而可用其 API Key 消耗租户余额。档位与动作不匹配，属于授权配置缺陷。
--
-- 存量补授规则：给【已持有 member:import 的角色】补 member:manage。
--   · member:import 本就在 member 模块的「操作」档，持有它的角色已经能创建成员，
--     补上「管理成员」不扩大其实际能力边界，行为保持连续；
--   · 只读档（仅 member:view，如预置的 support 角色）不补 —— 这正是本次要修的越权；
--   · 同时保证「操作/完全」档的角色在界面上仍然能反推出档位，不会退化成「自定义」。
--
-- 幂等：ON CONFLICT DO NOTHING，可重复执行；新装部署由 000020 的种子直接带上该权限点。

INSERT INTO sys_admin_role_permissions (role_id, permission_point)
SELECT DISTINCT rp.role_id, 'member:manage'
FROM sys_admin_role_permissions rp
WHERE rp.permission_point = 'member:import'
ON CONFLICT (role_id, permission_point) DO NOTHING;

-- 用户特批权限同理：特批过 member:import 的账号一并补 member:manage，
-- 否则依赖特批权限工作的账号会在升级后突然失去成员管理能力。
INSERT INTO sys_admin_role_perms (admin_user_id, permission_point)
SELECT DISTINCT p.admin_user_id, 'member:manage'
FROM sys_admin_role_perms p
WHERE p.permission_point = 'member:import'
ON CONFLICT DO NOTHING;

-- +goose Down
-- 回滚：移除该权限点。回滚后 rbac.go 若仍要求 member:manage，相关接口对非超管一律 403，
-- 因此代码与迁移需要同步回退。
DELETE FROM sys_admin_role_permissions WHERE permission_point = 'member:manage';
DELETE FROM sys_admin_role_perms WHERE permission_point = 'member:manage';
