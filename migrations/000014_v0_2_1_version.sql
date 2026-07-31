-- +goose Up
-- v0.2.1 变更汇总：
--   1. 修正 mdl_tenant_models.multiplier 字段注释（清理模型乘数误导，方案 B）
--   2. 新增细粒度用量日汇总表 bil_usage_daily（按租户/项目/模型/渠道/状态聚合，
--      供流量流向桑基图与模型性能分析；延迟以 SUM 存储，视图按 SUM/COUNT 求均值）
--   3. 移除被新表取代的旧粗粒度用量汇总表 bil_daily/monthly_usage_summary
--   4. 渠道调度重构（开发计划 §4.1，基线方案 §11）：chn_channels 新增 tier/strict_capacity，
--      chn_abilities 新增 cost_ratio，并按现有 priority 自动映射三档层级
--   5. 渠道调度重构收尾（阶段 5）：chn_channel_affinities 表标记废弃（会话绑定已迁移至 Redis）

-- 1. 修正 mdl_tenant_models.multiplier 注释
--
-- 原注释为「租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）」，
-- 把"模型倍率"写进了定价公式，易让人误以为该字段就是模型倍率。
-- 实际上：此字段是「租户倍率（tenant_multiplier）」来源之一，最终价格 = 基础价格 × 租户倍率。
-- （模型乘数 ModelMultiplier 当前为预留：mdl_models 无对应字段、computeCost 不纳入费用计算，恒为 1.0。）
COMMENT ON COLUMN mdl_tenant_models.multiplier IS '租户价格倍率（VIP 折扣）。作为 tenant_multiplier 参与最终价格计算：最终价格 = 基础价格 × 租户倍率。倍率来源优先级：discount_ratio > multiplier > 租户等级 price_multiplier';

-- 2. 新建细粒度用量日汇总表（由 usage_daily_aggregate 定时任务按天增量聚合 bil_usage_logs 写入）
CREATE TABLE bil_usage_daily (
    id                 BIGSERIAL       PRIMARY KEY,
    stat_date          DATE            NOT NULL,
    tenant_id          BIGINT          NOT NULL,
    project_id         BIGINT          NOT NULL DEFAULT 0,
    model_name         VARCHAR(128)    NOT NULL DEFAULT '',
    channel_id         BIGINT          NOT NULL DEFAULT 0,
    status             VARCHAR(20)     NOT NULL DEFAULT '',
    request_count      INTEGER         NOT NULL DEFAULT 0,
    input_tokens       BIGINT          NOT NULL DEFAULT 0,
    output_tokens      BIGINT          NOT NULL DEFAULT 0,
    total_cost         NUMERIC(20,10)  NOT NULL DEFAULT 0,
    account_cost       NUMERIC(20,10)  NOT NULL DEFAULT 0,
    sum_latency_ms     BIGINT          NOT NULL DEFAULT 0,
    sum_first_token_ms BIGINT          NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT uk_bil_usage_daily UNIQUE (stat_date, tenant_id, project_id, model_name, channel_id, status)
);

COMMENT ON TABLE bil_usage_daily IS '用量日维度汇总（按租户/项目/模型/渠道/状态聚合，供流量桑基图与模型性能分析使用）';
COMMENT ON COLUMN bil_usage_daily.stat_date IS '统计日期（按 created_at 的日期分桶）';
COMMENT ON COLUMN bil_usage_daily.tenant_id IS '租户ID';
COMMENT ON COLUMN bil_usage_daily.project_id IS '项目ID（0=无项目/个人Key）';
COMMENT ON COLUMN bil_usage_daily.model_name IS '模型名称';
COMMENT ON COLUMN bil_usage_daily.channel_id IS '渠道ID（0=无渠道）';
COMMENT ON COLUMN bil_usage_daily.status IS '请求状态：success/error/timeout/cancelled';
COMMENT ON COLUMN bil_usage_daily.request_count IS '请求数（COUNT(*)）';
COMMENT ON COLUMN bil_usage_daily.input_tokens IS '输入Token合计';
COMMENT ON COLUMN bil_usage_daily.output_tokens IS '输出Token合计';
COMMENT ON COLUMN bil_usage_daily.total_cost IS '客户侧成本合计（USD，源自 bil_usage_logs.total_cost）';
COMMENT ON COLUMN bil_usage_daily.account_cost IS '上游账户成本合计（USD，源自 bil_usage_logs.account_cost，用于利润分析）';
COMMENT ON COLUMN bil_usage_daily.sum_latency_ms IS '总延迟合计（ms，源自 bil_usage_logs.latency_ms；视图按 SUM/COUNT 求均值）';
COMMENT ON COLUMN bil_usage_daily.sum_first_token_ms IS '首Token延迟合计（ms，源自 bil_usage_logs.first_token_ms；视图按 SUM/COUNT 求均值）';

CREATE INDEX idx_bil_usage_daily_date ON bil_usage_daily (stat_date);
CREATE INDEX idx_bil_usage_daily_tenant_date ON bil_usage_daily (tenant_id, stat_date);

-- 3. 移除被新表取代的旧粗粒度用量汇总表（已废弃且无数据）
DROP TABLE IF EXISTS bil_daily_usage_summary;
DROP TABLE IF EXISTS bil_monthly_usage_summary;

-- 4. 渠道调度重构（开发计划 §4.1，基线方案 §11）：
--   4.1 tier：三档固定层级，替代自由整数优先级的调度语义（priority 列保留不删，仅停止参与调度，供回滚与历史查询）
--   4.2 strict_capacity：修订 R4，Redis 故障时该渠道 fail-closed（实例级保守限额），默认 fail-open
--   4.3 cost_ratio：渠道×模型成本比例（无量纲），参与调度 costFactor 计算

ALTER TABLE chn_channels ADD COLUMN IF NOT EXISTS tier VARCHAR(16) NOT NULL DEFAULT 'primary';
COMMENT ON COLUMN chn_channels.tier IS '调度层级：primary=首选 secondary=备用 reserve=保底';

ALTER TABLE chn_channels ADD COLUMN IF NOT EXISTS strict_capacity BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN chn_channels.strict_capacity IS '严格容量：true 时 Redis 故障期间使用实例级保守并发限额（fail-closed），用于高成本/严格配额渠道；false 为 fail-open';

ALTER TABLE chn_abilities ADD COLUMN IF NOT EXISTS cost_ratio NUMERIC(10,4) NOT NULL DEFAULT 1.0;
COMMENT ON COLUMN chn_abilities.cost_ratio IS '成本比例：该渠道该模型上游实际价/平台基准价，1.0=等价，0.8=八折，参与调度 costFactor 计算（无量纲比例，非金额）';

-- 按现有 priority 自动映射三档：全局最高优先级值 → primary，次高 → secondary，其余 → reserve
-- +goose StatementBegin
WITH ranked AS (
	SELECT id, DENSE_RANK() OVER (ORDER BY priority DESC) AS rk FROM chn_channels
)
UPDATE chn_channels c SET tier = CASE ranked.rk
	WHEN 1 THEN 'primary'
	WHEN 2 THEN 'secondary'
	ELSE 'reserve' END
FROM ranked WHERE c.id = ranked.id;
-- +goose StatementEnd

-- 5. 渠道调度重构收尾（阶段 5）：
-- chn_channel_affinities 表废弃——会话绑定已全部迁移至 Redis（dispatch:v1:bind:*），
-- 本表自旧调度（relay:affinity:v2 之前的 DB 方案）起已无代码读写，保留仅供历史查询。
-- 按计划标记废弃不删除，后续版本确认无审计需求后再物理清理。
COMMENT ON TABLE chn_channel_affinities IS '【已废弃 2026-07】渠道亲和性缓存。会话绑定已迁移至 Redis（dispatch:v1:bind:*），本表无代码读写，仅保留历史数据';

-- +goose Down
-- 回滚顺序与 Up 相反

-- 5. 恢复 chn_channel_affinities 表注释
COMMENT ON TABLE chn_channel_affinities IS '渠道亲和性缓存（用户+模型→渠道映射，TTL 1800s）';

-- 4. 移除渠道调度重构新增列
ALTER TABLE chn_abilities DROP COLUMN IF EXISTS cost_ratio;
ALTER TABLE chn_channels DROP COLUMN IF EXISTS strict_capacity;
ALTER TABLE chn_channels DROP COLUMN IF EXISTS tier;

-- 3. 重建旧粗粒度用量汇总表（仅恢复结构）
CREATE TABLE bil_daily_usage_summary (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT          NOT NULL,
    date            DATE            NOT NULL,
    total_requests  INT             NOT NULL DEFAULT 0,
    total_tokens    BIGINT          NOT NULL DEFAULT 0,
    total_cost      NUMERIC(20,10)  NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT uk_bil_daily_usage_summary UNIQUE (tenant_id, date)
);
COMMENT ON TABLE bil_daily_usage_summary IS '每日用量汇总（已废弃）';

CREATE TABLE bil_monthly_usage_summary (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT          NOT NULL,
    month           DATE            NOT NULL,
    total_requests  INT             NOT NULL DEFAULT 0,
    total_tokens    BIGINT          NOT NULL DEFAULT 0,
    total_cost      NUMERIC(20,10)  NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT uk_bil_monthly_usage_summary UNIQUE (tenant_id, month)
);
COMMENT ON TABLE bil_monthly_usage_summary IS '每月用量汇总（已废弃）';

-- 2. 删除细粒度用量日汇总表
DROP TABLE IF EXISTS bil_usage_daily;

-- 1. 恢复 mdl_tenant_models.multiplier 注释
COMMENT ON COLUMN mdl_tenant_models.multiplier IS '租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）';
