-- +goose Up

-- 1. 渠道×模型级协议能力标记（配合 Responses 端点支持）：
--   supports_responses：模型在该渠道支持 OpenAI Responses 协议（/v1/responses）。
--     responses 入站请求命中时原样直连上游 /v1/responses；调度时对 responses 入站软偏好此类渠道。
--   chat_via_responses：上游仅有 Responses 协议（responses-only，如 Codex 类中转），
--     chat 入站请求经 chat→Responses 桥接发送到 /v1/responses。
-- 两项均为渠道×模型粒度（非渠道级）：同一渠道的不同模型协议支持可能不同。

ALTER TABLE chn_abilities ADD COLUMN supports_responses BOOLEAN DEFAULT false NOT NULL;
COMMENT ON COLUMN chn_abilities.supports_responses IS '模型在该渠道支持 OpenAI Responses 协议（/v1/responses），responses 入站直连转发并获调度软偏好';

ALTER TABLE chn_abilities ADD COLUMN chat_via_responses BOOLEAN DEFAULT false NOT NULL;
COMMENT ON COLUMN chn_abilities.chat_via_responses IS '上游仅有 Responses 协议（responses-only 上游），chat 入站经桥接转换后发送 /v1/responses';

-- 2. 模型性能监控：bil_usage_daily 增加缓存聚合列（数据源自 bil_usage_logs 已有的缓存明细字段）。
--   cache_creation_tokens     缓存创建 token 数（Claude cache_creation_input_tokens）
--   cache_read_tokens         缓存命中读取 token 数（Claude cache_read / OpenAI cached_tokens）
--   cache_hit_request_count   命中缓存的请求数（明细行 cache_read_tokens > 0 计 1，与 request_count 同分母）
-- 上线后需用 aggregate-usage 命令回填历史区间（ON CONFLICT 幂等）。

ALTER TABLE bil_usage_daily ADD COLUMN cache_creation_tokens BIGINT DEFAULT 0 NOT NULL;
COMMENT ON COLUMN bil_usage_daily.cache_creation_tokens IS '缓存创建 token 数（Claude cache_creation / OpenAI Responses cache_write，SUM 聚合）';

ALTER TABLE bil_usage_daily ADD COLUMN cache_read_tokens BIGINT DEFAULT 0 NOT NULL;
COMMENT ON COLUMN bil_usage_daily.cache_read_tokens IS '缓存命中读取 token 数（Claude cache_read / OpenAI cached_tokens，SUM 聚合）';

ALTER TABLE bil_usage_daily ADD COLUMN cache_hit_request_count BIGINT DEFAULT 0 NOT NULL;
COMMENT ON COLUMN bil_usage_daily.cache_hit_request_count IS '命中缓存的请求数（明细 cache_read_tokens>0 计 1，含失败状态行）';

-- 3. 列语义注释同步更新（cache_creation_tokens 兼收 OpenAI Responses 的 cache_write_tokens）

COMMENT ON COLUMN bil_usage_logs.input_tokens IS '输入 token 数（含缓存总输入：base + cache_read + cache_creation，跨渠道统一口径）';
COMMENT ON COLUMN bil_usage_logs.cache_creation_tokens IS '写入缓存的 token 数（Claude cache_creation_input_tokens / OpenAI Responses cache_write_tokens）';
COMMENT ON COLUMN bil_usage_daily.input_tokens IS '输入Token合计（含缓存总输入，源自 bil_usage_logs.input_tokens）';

-- +goose Down

-- 回退注释（恢复原语义）
COMMENT ON COLUMN bil_usage_logs.input_tokens IS '输入 token 数（含缓存总输入：base + cache_read + cache_creation）';
COMMENT ON COLUMN bil_usage_logs.cache_creation_tokens IS '写入缓存的 token 数（Claude cache_creation_input_tokens）';
COMMENT ON COLUMN bil_usage_daily.input_tokens IS '输入Token合计（含缓存总输入，源自 bil_usage_logs.input_tokens）';

-- 删除缓存聚合列
ALTER TABLE bil_usage_daily DROP COLUMN IF EXISTS cache_creation_tokens;
ALTER TABLE bil_usage_daily DROP COLUMN IF EXISTS cache_read_tokens;
ALTER TABLE bil_usage_daily DROP COLUMN IF EXISTS cache_hit_request_count;

-- 删除协议能力标记列
ALTER TABLE chn_abilities DROP COLUMN IF EXISTS supports_responses;
ALTER TABLE chn_abilities DROP COLUMN IF EXISTS chat_via_responses;
