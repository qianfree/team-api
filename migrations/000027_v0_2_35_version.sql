-- +goose Up
-- ============================================================
-- bil_usage_daily 缓存创建 TTL 分桶聚合列
--
-- bil_usage_logs 早在 v0.2.x 已落库 cache_creation_5m_tokens /
-- cache_creation_1h_tokens（Claude cache_creation.ephemeral_5m/1h），
-- 日汇总仅有合并桶 cache_creation_tokens，TTL 维度无法观测
-- （5m/1h 计价不同：1h = 基准 ×1.6，分桶缺失时收益结构不可分析）。
--
--   cache_creation_5m_tokens   5 分钟 TTL 缓存创建 token（含 OpenAI cache_write
--                              与无 TTL 细分报文的兜底桶，SUM 聚合）
--   cache_creation_1h_tokens   1 小时 TTL 缓存创建 token（SUM 聚合）
--
-- 上线后需用 aggregate-usage 命令回填历史区间（ON CONFLICT 幂等）。

ALTER TABLE bil_usage_daily ADD COLUMN cache_creation_5m_tokens BIGINT DEFAULT 0 NOT NULL;
COMMENT ON COLUMN bil_usage_daily.cache_creation_5m_tokens IS '5分钟TTL缓存创建token数（Claude ephemeral_5m；OpenAI cache_write 并入此桶，SUM 聚合）';

ALTER TABLE bil_usage_daily ADD COLUMN cache_creation_1h_tokens BIGINT DEFAULT 0 NOT NULL;
COMMENT ON COLUMN bil_usage_daily.cache_creation_1h_tokens IS '1小时TTL缓存创建token数（Claude ephemeral_1h，SUM 聚合）';

-- +goose Down

ALTER TABLE bil_usage_daily DROP COLUMN cache_creation_5m_tokens;
ALTER TABLE bil_usage_daily DROP COLUMN cache_creation_1h_tokens;
