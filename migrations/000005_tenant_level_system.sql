-- +goose Up
-- 租户等级系统：基于累计充值的自动等级体系

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
(2, 'LV2', 100, 10, 20, 0.9500, 2),
(3, 'LV3', 500, 20, 50, 0.9000, 3),
(4, 'LV4', 2000, 50, 100, 0.8500, 4),
(5, 'LV5', 10000, 0, 0, 0.8000, 5);

-- 3. bil_wallets 添加累计充值字段
ALTER TABLE bil_wallets ADD COLUMN cumulative_recharge NUMERIC(20,10) DEFAULT 0 NOT NULL;
COMMENT ON COLUMN bil_wallets.cumulative_recharge IS '累计充值总额（USD）';

-- 4. tnt_tenants 添加等级字段
ALTER TABLE tnt_tenants ADD COLUMN level INT DEFAULT 1 NOT NULL;
COMMENT ON COLUMN tnt_tenants.level IS '当前等级（对应 tnt_tenant_level_configs.level）';

-- +goose Down
ALTER TABLE tnt_tenants DROP COLUMN IF EXISTS level;
ALTER TABLE bil_wallets DROP COLUMN IF EXISTS cumulative_recharge;
DROP TABLE IF EXISTS tnt_tenant_level_configs;
