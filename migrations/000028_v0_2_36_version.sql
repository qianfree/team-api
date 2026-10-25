-- +goose Up
-- ============================================================
-- OpenAI 兼容 /v1/files 素材上传接口支持列
--
-- fil_files 原以自增 id 对内标识文件；/v1/files 端点对外需暴露
-- OpenAI 风格的不可枚举文件 ID（file-xxx），并与内部数字 id 解耦，
-- 避免跨租户按序号探测。purpose 记录上传时声明的 OpenAI 用途
-- （vision/user_data/batch 等），供 GET /v1/files 列表过滤。
-- 两列仅 /v1/files 上传的文件写入（uploads/ 前缀），存量行为 NULL。
-- ============================================================

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
