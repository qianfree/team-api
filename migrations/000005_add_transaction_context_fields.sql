-- +goose Up
-- bil_transactions 增加关联字段，支持按用户、模型、请求维度查询和导出交易记录

ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS user_id BIGINT;
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS request_id VARCHAR(64);
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS model_name VARCHAR(100);

COMMENT ON COLUMN bil_transactions.user_id IS '关联用户ID（consume 类型为实际消费用户，recharge 类型为操作用户，adjust 类型为空）';
COMMENT ON COLUMN bil_transactions.request_id IS '关联请求ID（consume 类型对应 API 调用的 request_id，其他类型为空）';
COMMENT ON COLUMN bil_transactions.model_name IS '关联模型名（consume 类型为调用的模型名，其他类型为空）';

-- 按用户查询交易记录
CREATE INDEX IF NOT EXISTS idx_bil_transactions_user_created ON bil_transactions (tenant_id, user_id, created_at DESC);
-- 按模型查询消费记录
CREATE INDEX IF NOT EXISTS idx_bil_transactions_model_created ON bil_transactions (tenant_id, model_name, created_at DESC) WHERE model_name IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_bil_transactions_model_created;
DROP INDEX IF EXISTS idx_bil_transactions_user_created;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS model_name;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS request_id;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS user_id;
