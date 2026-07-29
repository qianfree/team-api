-- +goose Up
-- 新增细粒度用量日汇总表 bil_usage_daily（按 租户/项目/模型/渠道/状态 维度聚合），
-- 供流量流向桑基图与长周期用量趋势分析使用（替代旧粗粒度 bil_daily/monthly_usage_summary）。
-- 由 usage_daily_aggregate 定时任务按天增量聚合 bil_usage_logs 写入。

-- 1. 新建细粒度用量日汇总表
CREATE TABLE bil_usage_daily (
    id            BIGSERIAL       PRIMARY KEY,
    stat_date     DATE            NOT NULL,
    tenant_id     BIGINT          NOT NULL,
    project_id    BIGINT          NOT NULL DEFAULT 0,
    model_name    VARCHAR(128)    NOT NULL DEFAULT '',
    channel_id    BIGINT          NOT NULL DEFAULT 0,
    status        VARCHAR(20)     NOT NULL DEFAULT '',
    request_count INTEGER         NOT NULL DEFAULT 0,
    input_tokens  BIGINT          NOT NULL DEFAULT 0,
    output_tokens BIGINT          NOT NULL DEFAULT 0,
    total_cost    NUMERIC(20,10)  NOT NULL DEFAULT 0,
    account_cost  NUMERIC(20,10)  NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT uk_bil_usage_daily UNIQUE (stat_date, tenant_id, project_id, model_name, channel_id, status)
);

COMMENT ON TABLE bil_usage_daily IS '用量日维度汇总（按租户/项目/模型/渠道/状态聚合，供流量桑基图与趋势分析使用）';
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

CREATE INDEX idx_bil_usage_daily_date ON bil_usage_daily (stat_date);
CREATE INDEX idx_bil_usage_daily_tenant_date ON bil_usage_daily (tenant_id, stat_date);

-- 2. 移除被新表取代的旧粗粒度用量汇总表（已废弃且无数据）
DROP TABLE IF EXISTS bil_daily_usage_summary;
DROP TABLE IF EXISTS bil_monthly_usage_summary;

-- +goose Down
-- 回滚顺序与 Up 相反

-- 2. 重建旧粗粒度用量汇总表（仅恢复结构）
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

-- 1. 删除细粒度用量日汇总表
DROP TABLE IF EXISTS bil_usage_daily;
