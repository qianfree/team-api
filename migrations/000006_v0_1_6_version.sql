-- +goose Up
-- v0.1.6 变更汇总：
--   1. 创建 sys_cron_jobs 定时任务注册表
--   2. 删除 sys_cron_job_executions 旧表（改为单表管理）
--   3. 删除未使用的表：plg_example_logs、bil_daily/weekly/monthly_summary 系列
--   4. 修正 ord_orders.currency 默认值 USD→CNY，回填历史数据

-- 1. 创建定时任务注册表
CREATE TABLE sys_cron_jobs (
    id                BIGSERIAL PRIMARY KEY,
    job_name          VARCHAR(100) NOT NULL,
    schedule          VARCHAR(50) NOT NULL DEFAULT '',
    last_status       VARCHAR(20),
    last_started_at   TIMESTAMPTZ,
    last_finished_at  TIMESTAMPTZ,
    last_duration_ms  INTEGER,
    last_error_message TEXT,
    last_triggered_by VARCHAR(20),
    total_runs        INTEGER NOT NULL DEFAULT 0,
    total_failures    INTEGER NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE sys_cron_jobs IS '定时任务注册表';
COMMENT ON COLUMN sys_cron_jobs.job_name IS '任务名称（代码中定义的唯一标识）';
COMMENT ON COLUMN sys_cron_jobs.schedule IS 'cron 表达式';
COMMENT ON COLUMN sys_cron_jobs.last_status IS '最近一次执行状态：succeeded/failed';
COMMENT ON COLUMN sys_cron_jobs.last_started_at IS '最近一次开始执行时间';
COMMENT ON COLUMN sys_cron_jobs.last_finished_at IS '最近一次执行完成时间';
COMMENT ON COLUMN sys_cron_jobs.last_duration_ms IS '最近一次执行耗时（毫秒）';
COMMENT ON COLUMN sys_cron_jobs.last_error_message IS '最近一次错误信息（仅失败时有值）';
COMMENT ON COLUMN sys_cron_jobs.last_triggered_by IS '最近一次触发方式：auto/manual';
COMMENT ON COLUMN sys_cron_jobs.total_runs IS '累计执行次数';
COMMENT ON COLUMN sys_cron_jobs.total_failures IS '累计失败次数';

CREATE UNIQUE INDEX uk_sys_cron_jobs_name ON sys_cron_jobs (job_name);

-- 2. 删除旧的定时任务执行日志表
DROP TABLE IF EXISTS sys_cron_job_executions;

-- 3. 删除未使用的表
DROP TABLE IF EXISTS plg_example_logs;
DROP TABLE IF EXISTS bil_daily_usage_summary;
DROP TABLE IF EXISTS bil_daily_revenue_summary;
DROP TABLE IF EXISTS bil_monthly_usage_summary;
DROP TABLE IF EXISTS bil_monthly_revenue_summary;

-- 4. 修正订单层币种默认值：USD -> CNY
ALTER TABLE ord_orders ALTER COLUMN currency SET DEFAULT 'CNY';
COMMENT ON COLUMN ord_orders.currency IS '货币（订单层一律 CNY）';
UPDATE ord_orders SET currency = 'CNY' WHERE currency = 'USD';

-- +goose Down
-- 回滚顺序与 Up 相反

-- 4. 恢复 ord_orders.currency 默认值
ALTER TABLE ord_orders ALTER COLUMN currency SET DEFAULT 'USD';
COMMENT ON COLUMN ord_orders.currency IS '货币';

-- 3. 重建被删除的未使用表
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

-- 2. 重建 sys_cron_job_executions 表
CREATE TABLE sys_cron_job_executions (
    id                                       BIGSERIAL PRIMARY KEY,
    job_name                                 VARCHAR(100) NOT NULL,
    status                                   VARCHAR(20) NOT NULL,
    started_at                               TIMESTAMPTZ NOT NULL,
    finished_at                              TIMESTAMPTZ NOT NULL,
    duration_ms                              INTEGER NOT NULL,
    error_message                            TEXT,
    triggered_by                             VARCHAR(20) NOT NULL DEFAULT 'auto',
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);
CREATE INDEX idx_sys_cron_job_exec_name_time ON sys_cron_job_executions (job_name, created_at DESC);
CREATE INDEX idx_sys_cron_job_exec_created_brin ON sys_cron_job_executions USING BRIN (created_at);

-- 1. 删除 sys_cron_jobs 表
DROP TABLE IF EXISTS sys_cron_jobs;
