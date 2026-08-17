-- +goose Up
-- input_tokens 口径统一：bil_usage_logs.input_tokens 从此统一为「含缓存的总输入」。
--   历史问题：OpenAI/Gemini 渠道（promptTokenCount/prompt_tokens 原生含缓存）与 Claude 渠道
--   （input_tokens 原生不含缓存）写入同一列但语义不同，跨渠道 SUM 聚合与缓存命中率分母失真。
--   写侧已改为统一按 TotalInputTokens() 入库（Claude 口径补加 cache_read + cache_creation），
--   本迁移回填历史 Claude 渠道行（channel_type=2）使存量数据同口径。
-- 上线后需用 aggregate-usage 命令回填 bil_usage_daily 历史区间（ON CONFLICT 幂等）：
--   team-api aggregate-usage --from <最早用量日期> --to <今天>

UPDATE bil_usage_logs
SET input_tokens = input_tokens + COALESCE(cache_read_tokens, 0) + COALESCE(cache_creation_tokens, 0),
    total_tokens = total_tokens + COALESCE(cache_read_tokens, 0) + COALESCE(cache_creation_tokens, 0)
WHERE channel_type = 2
  AND (COALESCE(cache_read_tokens, 0) + COALESCE(cache_creation_tokens, 0)) > 0;

-- 列语义注释同步更新（cache_creation_tokens 兼收 OpenAI Responses 的 cache_write_tokens）
COMMENT ON COLUMN bil_usage_logs.input_tokens IS '输入 token 数（含缓存总输入：base + cache_read + cache_creation，跨渠道统一口径）';
COMMENT ON COLUMN bil_usage_logs.cache_creation_tokens IS '写入缓存的 token 数（Claude cache_creation_input_tokens / OpenAI Responses cache_write_tokens）';
COMMENT ON COLUMN bil_usage_daily.input_tokens IS '输入Token合计（含缓存总输入，源自 bil_usage_logs.input_tokens）';
COMMENT ON COLUMN bil_usage_daily.cache_creation_tokens IS '写入缓存的 token 数（Claude cache_creation / OpenAI cache_write，SUM 聚合）';

-- +goose Down
-- 逆转回填：Claude 渠道行恢复「不含缓存」口径（OpenAI/Gemini 行本就含缓存，不受影响）。
-- 注意：Down 后需再次执行 aggregate-usage 回滚 bil_usage_daily。

UPDATE bil_usage_logs
SET input_tokens = input_tokens - COALESCE(cache_read_tokens, 0) - COALESCE(cache_creation_tokens, 0),
    total_tokens = total_tokens - COALESCE(cache_read_tokens, 0) - COALESCE(cache_creation_tokens, 0)
WHERE channel_type = 2
  AND (COALESCE(cache_read_tokens, 0) + COALESCE(cache_creation_tokens, 0)) > 0;
