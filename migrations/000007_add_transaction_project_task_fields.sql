-- +goose Up
-- bil_transactions 增加项目和任务关联字段，支持按项目、密钥、任务维度追溯费用

ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS project_id BIGINT;
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS api_key_id BIGINT;
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS task_id VARCHAR(64);

COMMENT ON COLUMN bil_transactions.project_id IS '关联项目ID（consume 类型为 API Key 所属项目，个人密钥为空）';
COMMENT ON COLUMN bil_transactions.api_key_id IS '关联API密钥ID（consume 类型为发起请求的密钥）';
COMMENT ON COLUMN bil_transactions.task_id IS '关联异步任务公开ID（consume+relay_mode=task 时关联 tsk_model_tasks.public_task_id）';

-- 按项目查询消费记录
CREATE INDEX IF NOT EXISTS idx_bil_transactions_project_created ON bil_transactions (tenant_id, project_id, created_at DESC) WHERE project_id IS NOT NULL;
-- 按 API Key 查询消费记录
CREATE INDEX IF NOT EXISTS idx_bil_transactions_apikey_created ON bil_transactions (tenant_id, api_key_id, created_at DESC) WHERE api_key_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_bil_transactions_apikey_created;
DROP INDEX IF EXISTS idx_bil_transactions_project_created;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS task_id;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS api_key_id;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS project_id;
