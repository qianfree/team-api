-- +goose Up
-- 将资源包功能合并到套餐功能：改造 pln_plans 和 pln_tenant_plans，
-- 移除独立的 rsp_* 表和 pln_feature_flags 表

-- ============================================================
-- 1. 改造 pln_plans 表：加入资源包字段，移除订阅字段
-- ============================================================

-- 添加资源包字段
ALTER TABLE pln_plans ADD COLUMN credit_amount NUMERIC(20,10) DEFAULT 0 NOT NULL;
ALTER TABLE pln_plans ADD COLUMN bonus_amount NUMERIC(20,10) DEFAULT 0 NOT NULL;
ALTER TABLE pln_plans ADD COLUMN validity_days INTEGER DEFAULT 30 NOT NULL;
ALTER TABLE pln_plans ADD COLUMN model_group_id BIGINT DEFAULT 0;
ALTER TABLE pln_plans ADD COLUMN purchase_limit INTEGER DEFAULT 0 NOT NULL;
ALTER TABLE pln_plans ADD COLUMN purchase_limit_period VARCHAR(20) DEFAULT 'lifetime' NOT NULL;
ALTER TABLE pln_plans ADD COLUMN stock INTEGER;
ALTER TABLE pln_plans ADD COLUMN total_purchased INTEGER DEFAULT 0 NOT NULL;
ALTER TABLE pln_plans ADD COLUMN priority_access BOOLEAN DEFAULT false NOT NULL;
ALTER TABLE pln_plans ADD COLUMN advanced_log BOOLEAN DEFAULT false NOT NULL;
ALTER TABLE pln_plans ADD COLUMN support_level VARCHAR(20) DEFAULT 'standard' NOT NULL;

COMMENT ON COLUMN pln_plans.credit_amount IS '套餐包含的额度（USD）';
COMMENT ON COLUMN pln_plans.bonus_amount IS '赠送额度（USD）';
COMMENT ON COLUMN pln_plans.validity_days IS '有效天数，从激活时起算';
COMMENT ON COLUMN pln_plans.model_group_id IS '模型组ID，0=全部模型';
COMMENT ON COLUMN pln_plans.purchase_limit IS '限购数量，0=不限购';
COMMENT ON COLUMN pln_plans.purchase_limit_period IS '限购周期：lifetime/monthly/yearly';
COMMENT ON COLUMN pln_plans.stock IS '库存数量，NULL=不限';
COMMENT ON COLUMN pln_plans.total_purchased IS '累计购买次数';
COMMENT ON COLUMN pln_plans.priority_access IS '是否包含优先通道访问';
COMMENT ON COLUMN pln_plans.advanced_log IS '是否包含高级日志';
COMMENT ON COLUMN pln_plans.support_level IS '支持级别：standard/premium/dedicated';

-- monthly_price 重命名为 price
ALTER TABLE pln_plans RENAME COLUMN monthly_price TO price;
COMMENT ON COLUMN pln_plans.price IS '套餐价格（CNY）';

-- 移除订阅特有字段
ALTER TABLE pln_plans DROP COLUMN yearly_price;
ALTER TABLE pln_plans DROP COLUMN monthly_quota_tokens;
ALTER TABLE pln_plans DROP COLUMN allowed_models;

-- identifier 不再需要唯一约束（改为普通索引），但保留字段
ALTER TABLE pln_plans DROP CONSTRAINT uk_pln_plans_identifier;
CREATE INDEX idx_pln_plans_identifier ON pln_plans (identifier);

-- ============================================================
-- 2. 改造 pln_tenant_plans 表：加入额度字段，移除订阅字段
-- ============================================================

-- 添加额度字段
ALTER TABLE pln_tenant_plans ADD COLUMN total_credits NUMERIC(20,10) DEFAULT 0 NOT NULL;
ALTER TABLE pln_tenant_plans ADD COLUMN remaining_credits NUMERIC(20,10) DEFAULT 0 NOT NULL;
ALTER TABLE pln_tenant_plans ADD COLUMN paid_cny NUMERIC(20,10) DEFAULT 0;
ALTER TABLE pln_tenant_plans ADD COLUMN refunded_at TIMESTAMPTZ;

COMMENT ON COLUMN pln_tenant_plans.total_credits IS '总额度（USD）= credit_amount + bonus_amount';
COMMENT ON COLUMN pln_tenant_plans.remaining_credits IS '剩余额度（USD）';
COMMENT ON COLUMN pln_tenant_plans.paid_cny IS '实付金额（CNY）';
COMMENT ON COLUMN pln_tenant_plans.refunded_at IS '退款时间';

-- 添加 order_id 列（原 rsp_tenant_packages 有此列，pln_tenant_plans 没有）
ALTER TABLE pln_tenant_plans ADD COLUMN order_id BIGINT;
COMMENT ON COLUMN pln_tenant_plans.order_id IS '关联订单ID';

-- 移除订阅字段
ALTER TABLE pln_tenant_plans DROP COLUMN auto_renew;
ALTER TABLE pln_tenant_plans DROP COLUMN monthly_quota_tokens;
ALTER TABLE pln_tenant_plans DROP COLUMN used_tokens;
ALTER TABLE pln_tenant_plans DROP COLUMN last_reset_at;
ALTER TABLE pln_tenant_plans DROP COLUMN cancelled_at;

-- 添加计费查询索引
CREATE INDEX idx_pln_tenant_plans_billing ON pln_tenant_plans (tenant_id, status, end_at);
CREATE INDEX idx_pln_tenant_plans_order ON pln_tenant_plans (order_id);

-- ============================================================
-- 3. 重命名计费表中的资源包列
-- ============================================================

-- ord_orders: 删除 package_id（已有 plan_id 可用）
ALTER TABLE ord_orders DROP COLUMN IF EXISTS package_id;

-- bil_records: tenant_package_id → tenant_plan_id, package_deduction → plan_deduction
ALTER TABLE bil_records RENAME COLUMN tenant_package_id TO tenant_plan_id;
ALTER TABLE bil_records RENAME COLUMN package_deduction TO plan_deduction;

-- bil_transactions: tenant_package_id → tenant_plan_id
ALTER TABLE bil_transactions RENAME COLUMN tenant_package_id TO tenant_plan_id;

-- bil_usage_logs: tenant_package_id → tenant_plan_id, package_deduction → plan_deduction
ALTER TABLE bil_usage_logs RENAME COLUMN tenant_package_id TO tenant_plan_id;
ALTER TABLE bil_usage_logs RENAME COLUMN package_deduction TO plan_deduction;

-- ============================================================
-- 4. 删除不再需要的表
-- ============================================================

DROP TABLE IF EXISTS rsp_purchase_records;
DROP TABLE IF EXISTS rsp_tenant_packages;
DROP TABLE IF EXISTS rsp_packages;
DROP TABLE IF EXISTS pln_feature_flags;

-- +goose Down
-- 回滚：逆向操作

-- 4. 恢复删除的表（只有结构，不恢复数据）
CREATE TABLE pln_feature_flags (
    id               BIGSERIAL PRIMARY KEY,
    feature_key      VARCHAR(100) NOT NULL,
    description      TEXT,
    default_enabled  BOOLEAN DEFAULT false NOT NULL,
    enabled          BOOLEAN DEFAULT false NOT NULL,
    source           VARCHAR(20) DEFAULT 'manual' NOT NULL,
    source_id        BIGINT,
    tenant_id        BIGINT,
    plan_id          BIGINT,
    created_at       TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at       TIMESTAMPTZ DEFAULT now() NOT NULL
);

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
CREATE INDEX idx_rsp_tenant_packages_billing ON rsp_tenant_packages (tenant_id, status, expires_at);
CREATE INDEX idx_rsp_tenant_packages_order ON rsp_tenant_packages (order_id);

CREATE TABLE rsp_purchase_records (
    id               BIGSERIAL       PRIMARY KEY,
    tenant_id        BIGINT          NOT NULL,
    package_id       BIGINT          NOT NULL,
    order_id         BIGINT          NOT NULL,
    created_at       TIMESTAMPTZ     DEFAULT now() NOT NULL,
    CONSTRAINT uk_rsp_purchase_records UNIQUE (tenant_id, order_id)
);
CREATE INDEX idx_rsp_purchase_records_tenant_pkg ON rsp_purchase_records (tenant_id, package_id);

-- 3. 恢复计费表列名
ALTER TABLE bil_usage_logs RENAME COLUMN plan_deduction TO package_deduction;
ALTER TABLE bil_usage_logs RENAME COLUMN tenant_plan_id TO tenant_package_id;

ALTER TABLE bil_transactions RENAME COLUMN tenant_plan_id TO tenant_package_id;

ALTER TABLE bil_records RENAME COLUMN plan_deduction TO package_deduction;
ALTER TABLE bil_records RENAME COLUMN tenant_plan_id TO tenant_package_id;

ALTER TABLE ord_orders ADD COLUMN IF NOT EXISTS package_id BIGINT;

-- 2. 恢复 pln_tenant_plans
ALTER TABLE pln_tenant_plans DROP COLUMN IF EXISTS order_id;
DROP INDEX IF EXISTS idx_pln_tenant_plans_order;
DROP INDEX IF EXISTS idx_pln_tenant_plans_billing;
ALTER TABLE pln_tenant_plans DROP COLUMN IF EXISTS refunded_at;
ALTER TABLE pln_tenant_plans DROP COLUMN IF EXISTS paid_cny;
ALTER TABLE pln_tenant_plans DROP COLUMN IF EXISTS remaining_credits;
ALTER TABLE pln_tenant_plans DROP COLUMN IF EXISTS total_credits;
ALTER TABLE pln_tenant_plans ADD COLUMN cancelled_at TIMESTAMPTZ;
ALTER TABLE pln_tenant_plans ADD COLUMN last_reset_at TIMESTAMPTZ;
ALTER TABLE pln_tenant_plans ADD COLUMN used_tokens BIGINT DEFAULT 0 NOT NULL;
ALTER TABLE pln_tenant_plans ADD COLUMN monthly_quota_tokens BIGINT DEFAULT 0 NOT NULL;
ALTER TABLE pln_tenant_plans ADD COLUMN auto_renew BOOLEAN DEFAULT false NOT NULL;

-- 1. 恢复 pln_plans
ALTER TABLE pln_plans ADD COLUMN allowed_models TEXT[];
ALTER TABLE pln_plans ADD COLUMN monthly_quota_tokens BIGINT DEFAULT 0 NOT NULL;
ALTER TABLE pln_plans ADD COLUMN yearly_price NUMERIC(20,10) DEFAULT 0 NOT NULL;
ALTER TABLE pln_plans RENAME COLUMN price TO monthly_price;
DROP INDEX IF EXISTS idx_pln_plans_identifier;
ALTER TABLE pln_plans ADD CONSTRAINT uk_pln_plans_identifier UNIQUE (identifier);
ALTER TABLE pln_plans DROP COLUMN IF EXISTS support_level;
ALTER TABLE pln_plans DROP COLUMN IF EXISTS advanced_log;
ALTER TABLE pln_plans DROP COLUMN IF EXISTS priority_access;
ALTER TABLE pln_plans DROP COLUMN IF EXISTS total_purchased;
ALTER TABLE pln_plans DROP COLUMN IF EXISTS stock;
ALTER TABLE pln_plans DROP COLUMN IF EXISTS purchase_limit_period;
ALTER TABLE pln_plans DROP COLUMN IF EXISTS purchase_limit;
ALTER TABLE pln_plans DROP COLUMN IF EXISTS model_group_id;
ALTER TABLE pln_plans DROP COLUMN IF EXISTS validity_days;
ALTER TABLE pln_plans DROP COLUMN IF EXISTS bonus_amount;
ALTER TABLE pln_plans DROP COLUMN IF EXISTS credit_amount;
