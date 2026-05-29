-- +goose Up
-- 租户等级系统 + 模型分组功能 + 补充缺失索引

-- ============================================================
-- 一、租户等级系统：基于累计充值的自动等级体系
-- ============================================================

-- 1. 新建等级配置表
CREATE TABLE tnt_tenant_level_configs (
    id                              BIGSERIAL PRIMARY KEY,
    level                           INT NOT NULL,
    name                            VARCHAR(32) NOT NULL,
    cumulative_recharge_threshold   NUMERIC(20,10) NOT NULL,
    max_members                     INT NOT NULL DEFAULT 5,
    max_concurrency                 INT NOT NULL DEFAULT 10,
    price_multiplier                NUMERIC(5,4) NOT NULL DEFAULT 1.0000,
    sort_order                      INT NOT NULL DEFAULT 0,
    created_at                      TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                      TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_tnt_tenant_level_configs_level UNIQUE (level)
);

COMMENT ON TABLE tnt_tenant_level_configs IS '租户等级配置表';
COMMENT ON COLUMN tnt_tenant_level_configs.level IS '等级号（1, 2, 3...）';
COMMENT ON COLUMN tnt_tenant_level_configs.name IS '等级名称';
COMMENT ON COLUMN tnt_tenant_level_configs.cumulative_recharge_threshold IS '累计充值阈值（USD），达到此值自动升级';
COMMENT ON COLUMN tnt_tenant_level_configs.max_members IS '该等级最大成员数';
COMMENT ON COLUMN tnt_tenant_level_configs.max_concurrency IS '该等级最大并发数，0=无限';
COMMENT ON COLUMN tnt_tenant_level_configs.price_multiplier IS '价格乘数（折扣，如 0.9=九折）';
COMMENT ON COLUMN tnt_tenant_level_configs.sort_order IS '排序权重';

-- 2. 插入默认等级数据
INSERT INTO tnt_tenant_level_configs (level, name, cumulative_recharge_threshold, max_members, max_concurrency, price_multiplier, sort_order) VALUES
(1, 'LV1', 0, 5, 10, 1.0000, 1),
(2, 'LV2', 1000, 10, 20, 0.9500, 2),
(3, 'LV3', 10000, 20, 50, 0.9000, 3),
(4, 'LV4', 100000, 50, 100, 0.8500, 4),
(5, 'LV5', 500000, 0, 0, 0.8000, 5);

-- 3. bil_wallets 添加累计充值字段
ALTER TABLE bil_wallets ADD COLUMN cumulative_recharge NUMERIC(20,10) DEFAULT 0 NOT NULL;
COMMENT ON COLUMN bil_wallets.cumulative_recharge IS '累计充值总额（USD）';

-- 4. tnt_tenants 添加等级字段
ALTER TABLE tnt_tenants ADD COLUMN level INT DEFAULT 1 NOT NULL;
COMMENT ON COLUMN tnt_tenants.level IS '当前等级（对应 tnt_tenant_level_configs.level）';

-- ============================================================
-- 二、模型分组功能：通过分组批量管理租户可用的模型
-- ============================================================

-- 模型分组定义
CREATE TABLE mdl_model_groups (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    code        VARCHAR(50) NOT NULL,
    description TEXT,
    status      VARCHAR(20) DEFAULT 'active' NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at  TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_mdl_model_groups_code UNIQUE (code)
);

COMMENT ON TABLE mdl_model_groups IS '模型分组定义';
COMMENT ON COLUMN mdl_model_groups.id IS '主键ID';
COMMENT ON COLUMN mdl_model_groups.name IS '分组名称（如"全量模型"、"基础对话"）';
COMMENT ON COLUMN mdl_model_groups.code IS '分组唯一标识（如 full_access、basic_chat）';
COMMENT ON COLUMN mdl_model_groups.description IS '分组描述';
COMMENT ON COLUMN mdl_model_groups.status IS '状态：active（启用）/ disabled（禁用）';
COMMENT ON COLUMN mdl_model_groups.created_at IS '创建时间';
COMMENT ON COLUMN mdl_model_groups.updated_at IS '更新时间';

-- 分组-模型关联（多对多）
CREATE TABLE mdl_group_models (
    id         BIGSERIAL PRIMARY KEY,
    group_id   BIGINT NOT NULL,
    model_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_mdl_group_models UNIQUE (group_id, model_id)
);

CREATE INDEX idx_mdl_group_models_model_id ON mdl_group_models (model_id);

COMMENT ON TABLE mdl_group_models IS '分组-模型关联';
COMMENT ON COLUMN mdl_group_models.id IS '主键ID';
COMMENT ON COLUMN mdl_group_models.group_id IS '分组ID（关联 mdl_model_groups.id）';
COMMENT ON COLUMN mdl_group_models.model_id IS '模型ID（关联 mdl_models.id）';
COMMENT ON COLUMN mdl_group_models.created_at IS '创建时间';
COMMENT ON COLUMN mdl_group_models.updated_at IS '更新时间';

-- 租户-分组关联（多对多）
CREATE TABLE mdl_tenant_groups (
    id         BIGSERIAL PRIMARY KEY,
    tenant_id  BIGINT NOT NULL,
    group_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_mdl_tenant_groups UNIQUE (tenant_id, group_id)
);

CREATE INDEX idx_mdl_tenant_groups_tenant_id ON mdl_tenant_groups (tenant_id);

COMMENT ON TABLE mdl_tenant_groups IS '租户-分组关联';
COMMENT ON COLUMN mdl_tenant_groups.id IS '主键ID';
COMMENT ON COLUMN mdl_tenant_groups.tenant_id IS '租户ID（关联 tnt_tenants.id）';
COMMENT ON COLUMN mdl_tenant_groups.group_id IS '分组ID（关联 mdl_model_groups.id）';
COMMENT ON COLUMN mdl_tenant_groups.created_at IS '创建时间';
COMMENT ON COLUMN mdl_tenant_groups.updated_at IS '更新时间';

-- ============================================================
-- 三、补充缺失的外键列索引
-- ============================================================

CREATE INDEX idx_api_keys_user ON api_keys (user_id);
CREATE INDEX idx_bil_records_channel ON bil_records (channel_id) WHERE channel_id IS NOT NULL;
CREATE INDEX idx_fil_files_user ON fil_files (user_id);
CREATE INDEX idx_ord_promo_code_usages_user ON ord_promo_code_usages (user_id);
CREATE INDEX idx_ord_promo_code_usages_order ON ord_promo_code_usages (order_id);
CREATE INDEX idx_pln_tenant_plans_plan ON pln_tenant_plans (plan_id);
CREATE INDEX idx_spt_feedbacks_user ON spt_feedbacks (tenant_id, user_id);
CREATE INDEX idx_spt_replies_user ON spt_replies (user_id);

-- ============================================================
-- 四、租户 max_members 和 max_concurrency 改为可 NULL
-- NULL 表示跟随等级配置（tnt_tenant_level_configs），非 NULL 表示自定义覆盖
-- ============================================================

ALTER TABLE tnt_tenants ALTER COLUMN max_members DROP NOT NULL;
ALTER TABLE tnt_tenants ALTER COLUMN max_members SET DEFAULT NULL;
ALTER TABLE tnt_tenants ALTER COLUMN max_concurrency DROP NOT NULL;
ALTER TABLE tnt_tenants ALTER COLUMN max_concurrency SET DEFAULT NULL;

COMMENT ON COLUMN tnt_tenants.max_members IS '最大成员数上限（NULL表示跟随等级配置）';
COMMENT ON COLUMN tnt_tenants.max_concurrency IS '租户总并发上限（NULL表示跟随等级配置，0表示不限制）';

-- ============================================================
-- 五、sys_sessions 添加 jti 列（JWT ID，用于会话吊销）
-- ============================================================

ALTER TABLE sys_sessions ADD COLUMN jti VARCHAR(36) NOT NULL DEFAULT '';

UPDATE sys_sessions SET jti = 'legacy-' || id WHERE jti = '';

ALTER TABLE sys_sessions ADD CONSTRAINT uk_sys_sessions_jti UNIQUE (jti);

COMMENT ON COLUMN sys_sessions.jti IS 'JWT ID (jti)，会话唯一标识符（UUID），用于 Redis 吊销缓存';

-- +goose Down
-- 五、撤销 jti
ALTER TABLE sys_sessions DROP CONSTRAINT IF EXISTS uk_sys_sessions_jti;
ALTER TABLE sys_sessions DROP COLUMN IF EXISTS jti;

-- 四、恢复 max_members / max_concurrency 为 NOT NULL
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

-- 三、删除索引
DROP INDEX IF EXISTS idx_spt_replies_user;
DROP INDEX IF EXISTS idx_spt_feedbacks_user;
DROP INDEX IF EXISTS idx_pln_tenant_plans_plan;
DROP INDEX IF EXISTS idx_ord_promo_code_usages_order;
DROP INDEX IF EXISTS idx_ord_promo_code_usages_user;
DROP INDEX IF EXISTS idx_fil_files_user;
DROP INDEX IF EXISTS idx_bil_records_channel;
DROP INDEX IF EXISTS idx_api_keys_user;

-- 二、删除模型分组
DROP TABLE IF EXISTS mdl_tenant_groups;
DROP TABLE IF EXISTS mdl_group_models;
DROP TABLE IF EXISTS mdl_model_groups;

-- 一、删除租户等级
ALTER TABLE tnt_tenants DROP COLUMN IF EXISTS level;
ALTER TABLE bil_wallets DROP COLUMN IF EXISTS cumulative_recharge;
DROP TABLE IF EXISTS tnt_tenant_level_configs;
