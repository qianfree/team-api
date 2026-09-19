-- +goose Up
-- ============================================================
-- 日志表检索与排障优化
--
-- 覆盖 bil_usage_logs 与 aud_request_logs 两张日志表的索引补建
-- 与排障字段补充，详见各小节说明。
-- ============================================================


CREATE INDEX IF NOT EXISTS idx_bil_usage_logs_created_bt
    ON bil_usage_logs USING btree (created_at);

ALTER TABLE bil_usage_logs ADD COLUMN IF NOT EXISTS upstream_request_id VARCHAR(128) DEFAULT '' NOT NULL;
COMMENT ON COLUMN bil_usage_logs.upstream_request_id IS '上游请求ID：对话=上游响应头请求ID，任务=上游任务ID；排障反查用';
CREATE INDEX IF NOT EXISTS idx_bil_usage_logs_upstream_request_id
    ON bil_usage_logs USING btree (upstream_request_id);


ALTER TABLE aud_request_logs ADD COLUMN IF NOT EXISTS model_name VARCHAR(100);
COMMENT ON COLUMN aud_request_logs.model_name IS '请求使用的模型名（用户请求体中的原始模型，未经渠道映射）';
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_model
    ON aud_request_logs (model_name, created_at);
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_created_bt
    ON aud_request_logs USING btree (created_at);


-- +goose Down

-- 四、删除审计日志 created_at 单列索引
DROP INDEX IF EXISTS idx_aud_request_logs_created_bt;

-- 三、删除审计日志模型字段（独立审计库需同步手工回滚）
DROP INDEX IF EXISTS idx_aud_request_logs_model;
ALTER TABLE aud_request_logs DROP COLUMN IF EXISTS model_name;

-- 二、删除用量日志上游请求 ID 字段
DROP INDEX IF EXISTS idx_bil_usage_logs_upstream_request_id;
ALTER TABLE bil_usage_logs DROP COLUMN IF EXISTS upstream_request_id;

-- 一、删除用量日志 created_at 单列索引（分区父表删除会级联到各分区）
DROP INDEX IF EXISTS idx_bil_usage_logs_created_bt;
