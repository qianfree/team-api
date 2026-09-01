-- +goose Up
-- 管理后台角色权限管理：引入可自定义的角色实体
--
-- 背景：此前管理后台只有 super_admin / admin 两种身份，权限点直接绑在用户上
-- （sys_admin_role_perms 名为 role 实为 user_perms），无法复用、无法按岗位批量调整，
-- 结果是全团队共用超管账号。本迁移补齐「角色」这一层。
--
-- 角色模型：
--   · 超级管理员不入本表 —— 它由 sys_admin_users.role='super_admin' 唯一判定并在鉴权时短路，
--     特权判定只保留一个真相源；即便角色被删光，超管仍可登录并重建角色体系。
--   · 三个预置角色（管理员/运营/技术支持）只是安装时的初始数据，不是系统契约：
--     用户可以改权限、改名、禁用，也可以直接删掉自建一套。
--   · 有效权限 = ∪(已启用角色的权限) ∪ 用户特批权限(sys_admin_role_perms)，只做并集不做否定。

-- ── 角色定义 ──
CREATE TABLE IF NOT EXISTS sys_admin_roles (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(50)  NOT NULL,
    name        VARCHAR(50)  NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    is_builtin  BOOLEAN      NOT NULL DEFAULT FALSE,
    is_enabled  BOOLEAN      NOT NULL DEFAULT TRUE,
    sort        INT          NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uk_sys_admin_roles_code UNIQUE (code)
);

COMMENT ON TABLE sys_admin_roles IS '管理后台角色定义（超级管理员不入表，由 sys_admin_users.role 判定）';
COMMENT ON COLUMN sys_admin_roles.id IS '主键ID';
COMMENT ON COLUMN sys_admin_roles.code IS '角色标识（小写字母/数字/下划线，创建后不可修改，是权限缓存与审计日志的稳定标识；保留字 super_admin 不可占用）';
COMMENT ON COLUMN sys_admin_roles.name IS '角色显示名称（可修改）';
COMMENT ON COLUMN sys_admin_roles.description IS '角色说明，在分配界面展示';
COMMENT ON COLUMN sys_admin_roles.is_builtin IS '是否系统预置：仅用于标识来源与支撑「恢复默认权限」，不构成删除保护（预置角色同样可改可删）';
COMMENT ON COLUMN sys_admin_roles.is_enabled IS '是否启用：禁用后该角色的权限对全部关联用户立即失效，无需解除关联';
COMMENT ON COLUMN sys_admin_roles.sort IS '排序权重（升序）';
COMMENT ON COLUMN sys_admin_roles.created_at IS '创建时间';
COMMENT ON COLUMN sys_admin_roles.updated_at IS '更新时间';

-- ── 角色 → 权限点 ──
CREATE TABLE IF NOT EXISTS sys_admin_role_permissions (
    id               BIGSERIAL PRIMARY KEY,
    role_id          BIGINT       NOT NULL,
    permission_point VARCHAR(100) NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uk_sys_admin_role_permissions UNIQUE (role_id, permission_point)
);

CREATE INDEX IF NOT EXISTS idx_sys_admin_role_permissions_role ON sys_admin_role_permissions (role_id);

COMMENT ON TABLE sys_admin_role_permissions IS '角色权限点关联';
COMMENT ON COLUMN sys_admin_role_permissions.id IS '主键ID';
COMMENT ON COLUMN sys_admin_role_permissions.role_id IS '角色ID（逻辑关联 sys_admin_roles.id，无外键，删除角色时由业务层级联清理）';
COMMENT ON COLUMN sys_admin_role_permissions.permission_point IS '权限点标识（如 tenant:create、channel:edit）';
COMMENT ON COLUMN sys_admin_role_permissions.created_at IS '创建时间';
COMMENT ON COLUMN sys_admin_role_permissions.updated_at IS '更新时间';

-- ── 用户 → 角色（多对多，覆盖兼岗场景） ──
CREATE TABLE IF NOT EXISTS sys_admin_user_roles (
    id            BIGSERIAL PRIMARY KEY,
    admin_user_id BIGINT      NOT NULL,
    role_id       BIGINT      NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_sys_admin_user_roles UNIQUE (admin_user_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_sys_admin_user_roles_user ON sys_admin_user_roles (admin_user_id);
CREATE INDEX IF NOT EXISTS idx_sys_admin_user_roles_role ON sys_admin_user_roles (role_id);

COMMENT ON TABLE sys_admin_user_roles IS '管理员用户与角色关联（多对多：一个账号可同时担任多个角色，有效权限取并集）';
COMMENT ON COLUMN sys_admin_user_roles.id IS '主键ID';
COMMENT ON COLUMN sys_admin_user_roles.admin_user_id IS '管理员用户ID（逻辑关联 sys_admin_users.id，无外键）';
COMMENT ON COLUMN sys_admin_user_roles.role_id IS '角色ID（逻辑关联 sys_admin_roles.id，无外键）';
COMMENT ON COLUMN sys_admin_user_roles.created_at IS '创建时间';
COMMENT ON COLUMN sys_admin_user_roles.updated_at IS '更新时间';

-- ── 存量表语义调整：sys_admin_role_perms 由「用户权限」改为「用户特批权限」 ──
-- 数据不迁移、不删除。它现在表示「在角色之外额外授予该用户的权限」，
-- 保留「临时给某人单加一个权限」的口子，不必为一个权限点新建角色。
COMMENT ON TABLE sys_admin_role_perms IS '管理员用户特批权限（角色之外的额外授权，与角色权限取并集）';
COMMENT ON COLUMN sys_admin_role_perms.admin_user_id IS '关联的管理员用户ID';
COMMENT ON COLUMN sys_admin_role_perms.permission_point IS '特批的权限点标识（独立于角色，角色禁用不影响特批）';

-- ── 预置角色种子 ──
-- ON CONFLICT DO NOTHING 有两层作用：① 迁移可重复执行；
-- ② 用户改过或删过的预置角色，重跑迁移不会被覆盖或复活。
INSERT INTO sys_admin_roles (code, name, description, is_builtin, is_enabled, sort) VALUES
    ('admin',    '管理员',   '二把手：能碰钱（退款）与系统配置（支付、汇率、存储、邮件），不能管理账号与角色', TRUE, TRUE, 10),
    ('operator', '运营',     '平台日常运营：渠道、模型、租户、套餐、内容的完整权限，不涉及资金与系统设置', TRUE, TRUE, 20),
    ('support',  '技术支持', '排查客户问题：除财务数据外全站只读，唯一写权限是工单、反馈与帮助中心',       TRUE, TRUE, 30)
ON CONFLICT (code) DO NOTHING;

-- 管理员：除 user:*（账号与角色）、system:update/plugin 外的全部权限
INSERT INTO sys_admin_role_permissions (role_id, permission_point)
SELECT r.id, unnest(ARRAY[
    'dashboard:view',
    'channel:view','channel:create','channel:edit','channel:test','channel:delete',
    'model:view','model:create','model:edit','model:delete',
    'tenant:view','tenant:create','tenant:edit','tenant:suspend','tenant:delete','tenant:close',
    'member:view','member:import','member:model_scope',
    'plan:view','plan:create','plan:edit','plan:delete',
    'billing:view','billing:export','billing:refund',
    'order:view','order:refund',
    'promo:view','promo:create','promo:edit',
    'redemption:view','redemption:create','redemption:edit',
    'invoice:view','invoice:manage',
    'operation:view','operation:edit',
    'support:view','support:reply','support:edit',
    'monitor:view','monitor:edit',
    'task:view','task:edit',
    'file:view','file:delete','file:cleanup',
    'audit:view','audit:export',
    'system:view','system:edit'
])
FROM sys_admin_roles r WHERE r.code = 'admin'
ON CONFLICT (role_id, permission_point) DO NOTHING;

-- 运营：业务配置的完整权限；不给退款、审计、系统设置、账号管理
INSERT INTO sys_admin_role_permissions (role_id, permission_point)
SELECT r.id, unnest(ARRAY[
    'dashboard:view',
    'channel:view','channel:create','channel:edit','channel:test','channel:delete',
    'model:view','model:create','model:edit','model:delete',
    'tenant:view','tenant:create','tenant:edit','tenant:suspend',
    'member:view','member:import','member:model_scope',
    'plan:view','plan:create','plan:edit','plan:delete',
    'billing:view',
    'order:view',
    'promo:view','promo:create','promo:edit',
    'redemption:view','redemption:create','redemption:edit',
    'invoice:view','invoice:manage',
    'operation:view','operation:edit',
    'support:view','support:reply','support:edit',
    'monitor:view','monitor:edit',
    'task:view','task:edit',
    'file:view','file:delete','file:cleanup'
])
FROM sys_admin_roles r WHERE r.code = 'operator'
ON CONFLICT (role_id, permission_point) DO NOTHING;

-- 技术支持：除财务面（billing/order/promo/redemption/invoice）外全站只读 + 工单完整权限。
-- 排查主力工具不在财务面：请求审计日志走 audit:view，错误日志与渠道错误监控走 monitor:view。
INSERT INTO sys_admin_role_permissions (role_id, permission_point)
SELECT r.id, unnest(ARRAY[
    'dashboard:view',
    'channel:view',
    'model:view',
    'tenant:view',
    'member:view',
    'plan:view',
    'operation:view',
    'support:view','support:reply','support:edit',
    'monitor:view',
    'task:view',
    'file:view',
    'audit:view'
])
FROM sys_admin_roles r WHERE r.code = 'support'
ON CONFLICT (role_id, permission_point) DO NOTHING;

-- ── 存量账号回填 ──
-- 非超管账号一律挂到「管理员」角色，保持升级前后的可用权限不缩水
-- （升级前 admin 账号的实际权限来自 sys_admin_role_perms，特批权限继续并集生效）。
INSERT INTO sys_admin_user_roles (admin_user_id, role_id)
SELECT u.id, r.id
FROM sys_admin_users u
CROSS JOIN sys_admin_roles r
WHERE u.role <> 'super_admin' AND r.code = 'admin'
ON CONFLICT (admin_user_id, role_id) DO NOTHING;

-- +goose Down
-- 删除角色体系三张表。存量用户权限不受影响：它们本就存放在 sys_admin_role_perms，
-- 回滚后 HasPermission 退回「仅查用户特批权限」的旧语义。
DROP TABLE IF EXISTS sys_admin_user_roles;
DROP TABLE IF EXISTS sys_admin_role_permissions;
DROP TABLE IF EXISTS sys_admin_roles;

COMMENT ON TABLE sys_admin_role_perms IS '管理员角色权限关联';
COMMENT ON COLUMN sys_admin_role_perms.admin_user_id IS '关联的管理员用户ID';
COMMENT ON COLUMN sys_admin_role_perms.permission_point IS '权限点标识（如 tenant:create、channel:edit）';
