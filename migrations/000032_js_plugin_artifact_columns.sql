-- +goose Up
-- ============================================================
-- JS 插件表补全：js_task_artifacts 租户归属列 + 两表全量字段注释
--
--   · js_task_artifacts 补 tenant_id / user_id / channel_id（多租户表强约束；
--     同时是 sweep 恢复网的数据源——进程重启后重建 job 不再丢失归属，
--     相对 URL 产物按 channel_id 找渠道、fil_files 落正确租户）
--   · 存量行从 tsk_model_tasks 回填（幂等：仅回填 tenant_id=0 的行）
--   · 补齐 000031 缺失的字段 COMMENT（CLAUDE.md：所有字段必须带注释）

-- ── 1. js_task_artifacts 补列 ─────────────────────────────
-- +goose StatementBegin
ALTER TABLE js_task_artifacts ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE js_task_artifacts ADD COLUMN IF NOT EXISTS user_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE js_task_artifacts ADD COLUMN IF NOT EXISTS channel_id BIGINT NOT NULL DEFAULT 0;

-- 存量回填：从任务表取归属（tenant_id=0 的行即未回填过，可重复执行）
UPDATE js_task_artifacts a
SET tenant_id  = t.tenant_id,
    user_id    = t.user_id,
    channel_id = t.channel_id,
    updated_at = now()
FROM tsk_model_tasks t
WHERE a.task_id = t.id
  AND a.tenant_id = 0;

CREATE INDEX IF NOT EXISTS idx_js_task_artifacts_tenant ON js_task_artifacts (tenant_id);
-- +goose StatementEnd

COMMENT ON COLUMN js_task_artifacts.task_id IS '关联任务 ID（tsk_model_tasks.id，逻辑关联无外键）';
COMMENT ON COLUMN js_task_artifacts.public_task_id IS '公开任务 ID（task_xxx，存储 key 组成部分）';
COMMENT ON COLUMN js_task_artifacts.plugin_key IS '声明产物的 JS 插件 key';
COMMENT ON COLUMN js_task_artifacts.tenant_id IS '任务归属租户 ID（sweep 恢复与租户观测用，从任务表回填）';
COMMENT ON COLUMN js_task_artifacts.user_id IS '任务归属用户 ID（fil_files 落库归属）';
COMMENT ON COLUMN js_task_artifacts.channel_id IS '任务转发渠道 ID（相对 URL 产物按此解析渠道地址与鉴权）';
COMMENT ON COLUMN js_task_artifacts.artifact_key IS '产物键（任务内唯一，如 video / audio-0）';
COMMENT ON COLUMN js_task_artifacts.source_type IS '产物来源类型：url=下载转存 / inline=base64 直存';
COMMENT ON COLUMN js_task_artifacts.source_url IS '上游取值路径或下载 URL（声明期存 taskData 取值路径，转存时更新为真实 URL）';
COMMENT ON COLUMN js_task_artifacts.stored_key IS '对象存储 key（jsartifacts/{public_task_id}/{artifact_key}{ext}，stored 后非空）';
COMMENT ON COLUMN js_task_artifacts.backend IS '对象存储后端：s3/oss/cos/local';
COMMENT ON COLUMN js_task_artifacts.mime_type IS '存储内容类型';
COMMENT ON COLUMN js_task_artifacts.size IS '转存字节数';
COMMENT ON COLUMN js_task_artifacts.retry_count IS '已重试次数（<5 由 sweep 重试）';
COMMENT ON COLUMN js_task_artifacts.last_error IS '最近一次失败原因';
COMMENT ON COLUMN js_task_artifacts.created_at IS '创建时间';
COMMENT ON COLUMN js_task_artifacts.updated_at IS '更新时间';

-- ── 2. js_plugin_versions 补缺失注释 ──────────────────────
COMMENT ON COLUMN js_plugin_versions.id IS '主键';
COMMENT ON COLUMN js_plugin_versions.version IS '插件版本（semver，key+version 唯一）';
COMMENT ON COLUMN js_plugin_versions.source IS 'plugin.js 源码原文（上传/内置 embed）';
COMMENT ON COLUMN js_plugin_versions.source_hash IS '源码 SHA-256（同版本不可改内容的校验依据）';
COMMENT ON COLUMN js_plugin_versions.remark IS '管理端备注';
COMMENT ON COLUMN js_plugin_versions.created_at IS '创建时间';
COMMENT ON COLUMN js_plugin_versions.updated_at IS '更新时间';

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_js_task_artifacts_tenant;
ALTER TABLE js_task_artifacts DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE js_task_artifacts DROP COLUMN IF EXISTS user_id;
ALTER TABLE js_task_artifacts DROP COLUMN IF EXISTS channel_id;
-- +goose StatementEnd
