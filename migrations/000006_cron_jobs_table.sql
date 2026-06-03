-- +goose Up
-- 创建定时任务注册表，每条任务一行，记录最新执行状态和累计统计
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

-- +goose Down
DROP TABLE IF EXISTS sys_cron_jobs;
