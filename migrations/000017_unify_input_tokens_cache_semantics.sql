-- +goose Up

-- 列语义注释同步更新（cache_creation_tokens 兼收 OpenAI Responses 的 cache_write_tokens）
COMMENT ON COLUMN bil_usage_logs.input_tokens IS '输入 token 数（含缓存总输入：base + cache_read + cache_creation，跨渠道统一口径）';
COMMENT ON COLUMN bil_usage_logs.cache_creation_tokens IS '写入缓存的 token 数（Claude cache_creation_input_tokens / OpenAI Responses cache_write_tokens）';
COMMENT ON COLUMN bil_usage_daily.input_tokens IS '输入Token合计（含缓存总输入，源自 bil_usage_logs.input_tokens）';
COMMENT ON COLUMN bil_usage_daily.cache_creation_tokens IS '写入缓存的 token 数（Claude cache_creation / OpenAI cache_write，SUM 聚合）';

-- +goose Down
