-- +goose Up
-- 删除旧的定时任务执行日志表，改为 sys_cron_jobs 单表管理
DROP TABLE IF EXISTS sys_cron_job_executions;

-- +goose Down
-- 回滚：重建 sys_cron_job_executions 表
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
