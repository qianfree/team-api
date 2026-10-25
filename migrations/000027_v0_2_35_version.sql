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

-- ------------------------------------------------------------
-- OpenAI 兼容 /v1/files 素材上传接口支持列
--
-- fil_files 原以自增 id 对内标识文件；/v1/files 端点对外需暴露
-- OpenAI 风格的不可枚举文件 ID（file-xxx），并与内部数字 id 解耦，
-- 避免跨租户按序号探测。purpose 记录上传时声明的 OpenAI 用途
-- （vision/user_data/batch 等），供 GET /v1/files 列表过滤。
-- 两列仅 /v1/files 上传的文件写入（uploads/ 前缀），存量行为 NULL。
-- ------------------------------------------------------------

ALTER TABLE fil_files ADD COLUMN ext_id VARCHAR(64);
ALTER TABLE fil_files ADD COLUMN purpose VARCHAR(32);

COMMENT ON COLUMN fil_files.ext_id IS '对外文件 ID（file-<32hex>，OpenAI 兼容 /v1/files 专用，NULL=非用户上传素材）';
COMMENT ON COLUMN fil_files.purpose IS 'OpenAI 上传用途声明（vision/user_data/batch/assistants/fine-tune/evals），NULL=非 /v1/files 上传';

CREATE UNIQUE INDEX uq_fil_files_ext_id ON fil_files USING btree (ext_id) WHERE ext_id IS NOT NULL;
CREATE INDEX idx_fil_files_tenant_uploads ON fil_files USING btree (tenant_id, created_at) WHERE ext_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_fil_files_tenant_uploads;
DROP INDEX IF EXISTS uq_fil_files_ext_id;

ALTER TABLE fil_files
    DROP COLUMN IF EXISTS purpose,
    DROP COLUMN IF EXISTS ext_id;

ALTER TABLE bil_usage_daily DROP COLUMN cache_creation_5m_tokens;
ALTER TABLE bil_usage_daily DROP COLUMN cache_creation_1h_tokens;
