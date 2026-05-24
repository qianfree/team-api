-- +goose Up
-- 资源包功能：模型分组 + 资源包产品 + 租户持有包 + 购买记录
-- 同时修改订单、计费记录等表增加资源包关联字段

-- ============================================================
-- 1. 模型分组
-- ============================================================

-- 模型分组定义（如"GPT-4 系列"、"全部模型"）
CREATE TABLE mdl_model_groups (
    id               BIGSERIAL       PRIMARY KEY,
    name             VARCHAR(100)    NOT NULL,
    description      TEXT,
    sort_order       INTEGER         DEFAULT 0 NOT NULL,
    status           VARCHAR(20)     DEFAULT 'active' NOT NULL,
    created_at       TIMESTAMPTZ     DEFAULT now() NOT NULL,
    updated_at       TIMESTAMPTZ     DEFAULT now() NOT NULL
);

COMMENT ON TABLE mdl_model_groups IS '模型分组定义，用于资源包的模型范围管理';

-- 分组-模型关联
CREATE TABLE mdl_model_group_items (
    id               BIGSERIAL       PRIMARY KEY,
    group_id         BIGINT          NOT NULL,
    model_name       VARCHAR(100)    NOT NULL,
    created_at       TIMESTAMPTZ     DEFAULT now() NOT NULL,
    CONSTRAINT fk_mdl_model_group_items_group FOREIGN KEY (group_id) REFERENCES mdl_model_groups(id) ON DELETE CASCADE,
    CONSTRAINT uk_mdl_model_group_items UNIQUE (group_id, model_name)
);

COMMENT ON TABLE mdl_model_group_items IS '模型分组项，关联分组与具体模型';

CREATE INDEX idx_mdl_model_group_items_group ON mdl_model_group_items (group_id);

-- ============================================================
-- 2. 资源包产品定义
-- ============================================================

CREATE TABLE rsp_packages (
    id                    BIGSERIAL       PRIMARY KEY,
    name                  VARCHAR(100)    NOT NULL,
    description           TEXT,
    price                 NUMERIC(20,10)  NOT NULL,
    credit_amount         NUMERIC(20,10)  NOT NULL,
    bonus_amount          NUMERIC(20,10)  DEFAULT 0 NOT NULL,
    validity_days         INTEGER         NOT NULL,
    model_group_id        BIGINT,
    purchase_limit        INTEGER         DEFAULT 0 NOT NULL,
    purchase_limit_period VARCHAR(20)     DEFAULT 'lifetime' NOT NULL,
    stock                 INTEGER,
    total_purchased       INTEGER         DEFAULT 0 NOT NULL,
    priority_access       BOOLEAN         DEFAULT false NOT NULL,
    advanced_log          BOOLEAN         DEFAULT false NOT NULL,
    support_level         VARCHAR(20)     DEFAULT 'standard' NOT NULL,
    sort_order            INTEGER         DEFAULT 0 NOT NULL,
    status                VARCHAR(20)     DEFAULT 'active' NOT NULL,
    created_at            TIMESTAMPTZ     DEFAULT now() NOT NULL,
    updated_at            TIMESTAMPTZ     DEFAULT now() NOT NULL
);

COMMENT ON TABLE rsp_packages IS '资源包产品定义';

-- ============================================================
-- 3. 租户持有的资源包（可叠加）
-- ============================================================

CREATE TABLE rsp_tenant_packages (
    id                  BIGSERIAL       PRIMARY KEY,
    tenant_id           BIGINT          NOT NULL,
    package_id          BIGINT          NOT NULL,
    order_id            BIGINT,
    status              VARCHAR(20)     DEFAULT 'active' NOT NULL,
    total_credits       NUMERIC(20,10)  NOT NULL,
    remaining_credits   NUMERIC(20,10)  NOT NULL,
    paid_cny            NUMERIC(20,10)  NOT NULL,
    activated_at        TIMESTAMPTZ     NOT NULL,
    expires_at          TIMESTAMPTZ     NOT NULL,
    expired_at          TIMESTAMPTZ,
    refunded_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ     DEFAULT now() NOT NULL,
    updated_at          TIMESTAMPTZ     DEFAULT now() NOT NULL
);

COMMENT ON TABLE rsp_tenant_packages IS '租户持有的资源包，多个可同时有效';

-- 计费查询核心索引：按租户查有效包，按过期时间排序
CREATE INDEX idx_rsp_tenant_packages_billing ON rsp_tenant_packages (tenant_id, status, expires_at);
CREATE INDEX idx_rsp_tenant_packages_order ON rsp_tenant_packages (order_id);

-- ============================================================
-- 4. 购买记录（限购计数用）
-- ============================================================

CREATE TABLE rsp_purchase_records (
    id               BIGSERIAL       PRIMARY KEY,
    tenant_id        BIGINT          NOT NULL,
    package_id       BIGINT          NOT NULL,
    order_id         BIGINT          NOT NULL,
    created_at       TIMESTAMPTZ     DEFAULT now() NOT NULL,
    CONSTRAINT uk_rsp_purchase_records UNIQUE (tenant_id, order_id)
);

COMMENT ON TABLE rsp_purchase_records IS '资源包购买记录，用于限购计数';

CREATE INDEX idx_rsp_purchase_records_tenant_pkg ON rsp_purchase_records (tenant_id, package_id);

-- ============================================================
-- 5. 修改现有表
-- ============================================================

-- 订单表增加资源包关联
ALTER TABLE ord_orders ADD COLUMN IF NOT EXISTS package_id BIGINT;

-- 计费记录增加资源包扣费来源
ALTER TABLE bil_records ADD COLUMN IF NOT EXISTS tenant_package_id BIGINT;
ALTER TABLE bil_records ADD COLUMN IF NOT EXISTS package_deduction NUMERIC(20,10) DEFAULT 0 NOT NULL;
ALTER TABLE bil_records ADD COLUMN IF NOT EXISTS wallet_deduction NUMERIC(20,10) DEFAULT 0 NOT NULL;

-- 交易流水增加资源包关联
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS tenant_package_id BIGINT;

-- 使用日志增加资源包扣费来源
ALTER TABLE bil_usage_logs ADD COLUMN IF NOT EXISTS tenant_package_id BIGINT;
ALTER TABLE bil_usage_logs ADD COLUMN IF NOT EXISTS package_deduction NUMERIC(20,10) DEFAULT 0 NOT NULL;
ALTER TABLE bil_usage_logs ADD COLUMN IF NOT EXISTS wallet_deduction NUMERIC(20,10) DEFAULT 0 NOT NULL;

-- ============================================================
-- 6. 初始化默认模型组
-- ============================================================

-- 插入一个"全部模型"默认分组
INSERT INTO mdl_model_groups (name, description, sort_order, status) VALUES
    ('全部模型', '包含所有可用模型', 0, 'active');

-- +goose Down
-- 回滚：按逆序删除

ALTER TABLE bil_usage_logs DROP COLUMN IF EXISTS wallet_deduction;
ALTER TABLE bil_usage_logs DROP COLUMN IF EXISTS package_deduction;
ALTER TABLE bil_usage_logs DROP COLUMN IF EXISTS tenant_package_id;

ALTER TABLE bil_transactions DROP COLUMN IF EXISTS tenant_package_id;

ALTER TABLE bil_records DROP COLUMN IF EXISTS wallet_deduction;
ALTER TABLE bil_records DROP COLUMN IF EXISTS package_deduction;
ALTER TABLE bil_records DROP COLUMN IF EXISTS tenant_package_id;

ALTER TABLE ord_orders DROP COLUMN IF EXISTS package_id;

DROP TABLE IF EXISTS rsp_purchase_records;
DROP TABLE IF EXISTS rsp_tenant_packages;
DROP TABLE IF EXISTS rsp_packages;
DROP TABLE IF EXISTS mdl_model_group_items;
DROP TABLE IF EXISTS mdl_model_groups;
