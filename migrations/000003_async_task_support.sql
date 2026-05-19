-- +goose Up
-- 异步任务支持：审计日志任务字段 + 用量日志任务字段 + 任务表request_id + 请求类型注释更新

-- 1. aud_request_logs 新增任务结果相关字段
ALTER TABLE aud_request_logs
    ADD COLUMN IF NOT EXISTS task_id               VARCHAR(64),
    ADD COLUMN IF NOT EXISTS task_status           VARCHAR(20),
    ADD COLUMN IF NOT EXISTS task_result           TEXT,
    ADD COLUMN IF NOT EXISTS task_upstream_headers JSONB,
    ADD COLUMN IF NOT EXISTS task_completed_at     TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_aud_request_logs_task_id
    ON aud_request_logs (task_id) WHERE task_id IS NOT NULL;

COMMENT ON COLUMN aud_request_logs.task_id IS '异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id';
COMMENT ON COLUMN aud_request_logs.task_status IS '异步任务终态：SUCCESS / FAILURE';
COMMENT ON COLUMN aud_request_logs.task_result IS '异步任务完成时上游返回的原始响应体';
COMMENT ON COLUMN aud_request_logs.task_upstream_headers IS '异步任务完成时上游返回的响应头（仅审计级别为 full 时记录）';
COMMENT ON COLUMN aud_request_logs.task_completed_at IS '异步任务达到终态的时间';

-- 2. bil_usage_logs 新增 task_id 字段
ALTER TABLE bil_usage_logs
    ADD COLUMN IF NOT EXISTS task_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_bil_usage_logs_task_id
    ON bil_usage_logs (task_id) WHERE task_id IS NOT NULL;

COMMENT ON COLUMN bil_usage_logs.task_id IS '异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id，普通请求为空';

-- 3. bil_usage_logs.request_type 注释更新（新增 async=3）
COMMENT ON COLUMN bil_usage_logs.request_type IS '请求类型: 1=sync, 2=stream, 3=async, 4=websocket';

-- 4. tsk_model_tasks 新增 request_id 字段，存储任务提交时的原始请求 ID
ALTER TABLE tsk_model_tasks
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_tsk_model_tasks_request_id
    ON tsk_model_tasks (request_id) WHERE request_id IS NOT NULL;

COMMENT ON COLUMN tsk_model_tasks.request_id IS '任务提交时的原始请求 ID（req_xxxxx），关联 aud_request_logs.request_id';

-- +goose Down
DROP INDEX IF EXISTS idx_tsk_model_tasks_request_id;
ALTER TABLE tsk_model_tasks
    DROP COLUMN IF EXISTS request_id;

DROP INDEX IF EXISTS idx_aud_request_logs_task_id;
ALTER TABLE aud_request_logs
    DROP COLUMN IF EXISTS task_id,
    DROP COLUMN IF EXISTS task_status,
    DROP COLUMN IF EXISTS task_result,
    DROP COLUMN IF EXISTS task_upstream_headers,
    DROP COLUMN IF EXISTS task_completed_at;

DROP INDEX IF EXISTS idx_bil_usage_logs_task_id;
ALTER TABLE bil_usage_logs
    DROP COLUMN IF EXISTS task_id;

COMMENT ON COLUMN bil_usage_logs.request_type IS '请求类型: 1=sync, 2=stream, 3=websocket';
