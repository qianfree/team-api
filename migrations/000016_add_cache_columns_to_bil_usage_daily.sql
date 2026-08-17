-- +goose Up
-- 模型性能监控：bil_usage_daily 增加缓存聚合列（数据源自 bil_usage_logs 已有的缓存明细字段）。
--   cache_creation_tokens     缓存创建 token 数（Claude cache_creation_input_tokens）
--   cache_read_tokens         缓存命中读取 token 数（Claude cache_read / OpenAI cached_tokens）
--   cache_hit_request_count   命中缓存的请求数（明细行 cache_read_tokens > 0 计 1，与 request_count 同分母）
-- 上线后需用 aggregate-usage 命令回填历史区间（ON CONFLICT 幂等）。

ALTER TABLE bil_usage_daily ADD COLUMN cache_creation_tokens BIGINT DEFAULT 0 NOT NULL;
COMMENT ON COLUMN bil_usage_daily.cache_creation_tokens IS '缓存创建 token 数（Claude cache_creation，SUM 聚合）';

ALTER TABLE bil_usage_daily ADD COLUMN cache_read_tokens BIGINT DEFAULT 0 NOT NULL;
COMMENT ON COLUMN bil_usage_daily.cache_read_tokens IS '缓存命中读取 token 数（Claude cache_read / OpenAI cached_tokens，SUM 聚合）';

ALTER TABLE bil_usage_daily ADD COLUMN cache_hit_request_count BIGINT DEFAULT 0 NOT NULL;
COMMENT ON COLUMN bil_usage_daily.cache_hit_request_count IS '命中缓存的请求数（明细 cache_read_tokens>0 计 1，含失败状态行）';

-- +goose Down
ALTER TABLE bil_usage_daily DROP COLUMN IF EXISTS cache_creation_tokens;
ALTER TABLE bil_usage_daily DROP COLUMN IF EXISTS cache_read_tokens;
ALTER TABLE bil_usage_daily DROP COLUMN IF EXISTS cache_hit_request_count;
