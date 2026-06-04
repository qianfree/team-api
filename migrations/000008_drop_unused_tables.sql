-- +goose Up
-- 删除未使用的表：
--   plg_example_logs：插件示例日志，代码中零引用
--   bil_daily_usage_summary / bil_daily_revenue_summary：日汇总，只有写入无读取
--   bil_monthly_usage_summary / bil_monthly_revenue_summary：月汇总，只有写入无读取

DROP TABLE IF EXISTS plg_example_logs;
DROP TABLE IF EXISTS bil_daily_usage_summary;
DROP TABLE IF EXISTS bil_daily_revenue_summary;
DROP TABLE IF EXISTS bil_monthly_usage_summary;
DROP TABLE IF EXISTS bil_monthly_revenue_summary;

-- +goose Down
-- 回滚：重建被删除的表

CREATE TABLE plg_example_logs (
    id          BIGSERIAL PRIMARY KEY,
    plugin_name VARCHAR(100) NOT NULL,
    tenant_id   BIGINT,
    message     TEXT,
    level       VARCHAR(20) DEFAULT 'info',
    created_at  TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at  TIMESTAMPTZ DEFAULT now() NOT NULL
);
COMMENT ON TABLE plg_example_logs IS '插件示例日志表（已废弃）';

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

CREATE TABLE bil_daily_revenue_summary (
    id                 BIGSERIAL PRIMARY KEY,
    date               DATE            NOT NULL,
    total_recharge     NUMERIC(20,10)  NOT NULL DEFAULT 0,
    total_consumption  NUMERIC(20,10)  NOT NULL DEFAULT 0,
    net_revenue        NUMERIC(20,10)  NOT NULL DEFAULT 0,
    new_orders         INT             NOT NULL DEFAULT 0,
    paid_orders        INT             NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT uk_bil_daily_revenue_summary UNIQUE (date)
);
COMMENT ON TABLE bil_daily_revenue_summary IS '每日收入汇总（已废弃）';

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

CREATE TABLE bil_monthly_revenue_summary (
    id                 BIGSERIAL PRIMARY KEY,
    month              DATE            NOT NULL,
    total_recharge     NUMERIC(20,10)  NOT NULL DEFAULT 0,
    total_consumption  NUMERIC(20,10)  NOT NULL DEFAULT 0,
    net_revenue        NUMERIC(20,10)  NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT uk_bil_monthly_revenue_summary UNIQUE (month)
);
COMMENT ON TABLE bil_monthly_revenue_summary IS '每月收入汇总（已废弃）';
