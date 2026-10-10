-- +goose Up
-- ============================================================
-- 请求审计日志会话关联列
--
-- 无状态对话协议下客户端每轮重发全部历史，N 轮对话产生 N 条
-- 相互重叠的审计记录。新增会话关联列支持把同一会话的记录分组
-- 串联并一键还原完整对话（设计见 docs/会话关联与完整会话提取方案.md）：
--   session_key/session_source   显式会话信号（头/协议字段）派生键，仅精确来源写入
--   conv_fingerprint             会话头部指纹（system+首条 user 消息规范化哈希）
--   resp_msg_hash                响应 assistant 消息哈希，衔接链前驱查询用
--   last_asst_msg_hash           请求 messages 末条 assistant 消息哈希，
--                                与上一轮 resp_msg_hash 相等即两轮衔接
--   is_context_reset             写时检测到上下文压缩/重写标记
--   user_field                   OpenAI user 字段原文（hint，不参与键解析）
--
-- 哈希均在原始请求/响应体上计算（先于审计级别处理与截断），
-- 关联能力不受审计级别与 2 KiB 截断影响。
-- ============================================================

ALTER TABLE aud_request_logs ADD COLUMN session_key VARCHAR(128);
ALTER TABLE aud_request_logs ADD COLUMN session_source VARCHAR(20);
ALTER TABLE aud_request_logs ADD COLUMN conv_fingerprint VARCHAR(80);
ALTER TABLE aud_request_logs ADD COLUMN resp_msg_hash VARCHAR(80);
ALTER TABLE aud_request_logs ADD COLUMN last_asst_msg_hash VARCHAR(80);
ALTER TABLE aud_request_logs ADD COLUMN is_context_reset BOOLEAN DEFAULT FALSE NOT NULL;
ALTER TABLE aud_request_logs ADD COLUMN user_field VARCHAR(255);

COMMENT ON COLUMN aud_request_logs.session_key IS '会话关联键（sk:来源:sha256），仅显式会话信号（X-Session-Id 头/Claude metadata.user_id/Responses 链）命中时写入，NULL=无显式键';
COMMENT ON COLUMN aud_request_logs.session_source IS '会话键来源：header/anthropic/openai，NULL=无显式键';
COMMENT ON COLUMN aud_request_logs.conv_fingerprint IS '会话指纹 fp1:sha256（system+首条user消息规范化哈希），同会话各轮共享，内容分组用';
COMMENT ON COLUMN aud_request_logs.resp_msg_hash IS '本条响应 assistant 消息规范化哈希 mh1:sha256，衔接链前驱查询用';
COMMENT ON COLUMN aud_request_logs.last_asst_msg_hash IS '本条请求 messages 最后一条 assistant 消息规范化哈希 mh1:sha256，与上一轮 resp_msg_hash 相等即两轮衔接';
COMMENT ON COLUMN aud_request_logs.is_context_reset IS '写时检测到上下文压缩/重写标记（best-effort，权威判定在会话重组查询时按消息量骤降判定）';
COMMENT ON COLUMN aud_request_logs.user_field IS '请求体 user 字段原文（OpenAI 协议 hint，不参与会话键解析）';

-- 会话视图查询：按租户过滤后按会话键/指纹取时间有序记录；resp_hash 为衔接链前驱点查
CREATE INDEX idx_aud_request_logs_session ON aud_request_logs USING btree (tenant_id, session_key, created_at) WHERE session_key IS NOT NULL;
CREATE INDEX idx_aud_request_logs_conv_fp ON aud_request_logs USING btree (tenant_id, conv_fingerprint, created_at) WHERE conv_fingerprint IS NOT NULL;
CREATE INDEX idx_aud_request_logs_resp_hash ON aud_request_logs USING btree (tenant_id, resp_msg_hash) WHERE resp_msg_hash IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_aud_request_logs_resp_hash;
DROP INDEX IF EXISTS idx_aud_request_logs_conv_fp;
DROP INDEX IF EXISTS idx_aud_request_logs_session;

ALTER TABLE aud_request_logs
    DROP COLUMN IF EXISTS user_field,
    DROP COLUMN IF EXISTS is_context_reset,
    DROP COLUMN IF EXISTS last_asst_msg_hash,
    DROP COLUMN IF EXISTS resp_msg_hash,
    DROP COLUMN IF EXISTS conv_fingerprint,
    DROP COLUMN IF EXISTS session_source,
    DROP COLUMN IF EXISTS session_key;
