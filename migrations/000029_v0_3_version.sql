-- +goose Up
-- ============================================================
-- v0.3 插件体系：JS 异步任务插件数据表
--
--   · js_plugin_versions   插件版本表（源码 + 编译产物 + manifest + origin/icon）
--   · js_task_artifacts    产物转存记录表（声明 → 下载 → 对象存储 → URL 改写）
--
-- 幂等性（守卫，任何部署状态重跑均无操作）：表不存在 → 新建（列定义
-- 齐全即终态）；旧版形态表已存在 → ADD COLUMN IF NOT EXISTS 补列；
-- 列已存在 → 跳过。设计见 docs/JS插件体系架构与调用链分析.md。
-- ============================================================

-- ── 1. 插件版本表 ──────────────────────────────────────────
-- 插件以源码形态上传（plugin.js），key+version 唯一，多版本共存，
-- 每个插件 key 至多一个 active 版本；enabled 为插件级开关（挂在 active 版本行上）。
-- origin：builtin=内置（go:embed 随二进制，不可删除）/ upload=管理端上传；
-- icon：品牌图标 data URI（SVG/PNG，经安全校验，读取走 GET /api/admin/js-plugins/{key}/icon）。

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS js_plugin_versions (
    id           BIGSERIAL PRIMARY KEY,
    plugin_key   VARCHAR(64)  NOT NULL,
    version      VARCHAR(64)  NOT NULL,
    source       TEXT         NOT NULL,
    source_hash  VARCHAR(64)  NOT NULL,
    compiled     TEXT,
    manifest     JSONB        NOT NULL,
    origin       VARCHAR(16)  NOT NULL DEFAULT 'upload',
    icon         TEXT,
    enabled      BOOLEAN      NOT NULL DEFAULT FALSE,
    active       BOOLEAN      NOT NULL DEFAULT FALSE,
    remark       TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_js_plugin_key_version ON js_plugin_versions (plugin_key, version);
CREATE INDEX IF NOT EXISTS idx_js_plugin_active ON js_plugin_versions (plugin_key) WHERE active;

-- 幂等守卫：旧版形态表（列缺失）补列，列已存在为无操作
ALTER TABLE js_plugin_versions ADD COLUMN IF NOT EXISTS origin VARCHAR(16) NOT NULL DEFAULT 'upload';
ALTER TABLE js_plugin_versions ADD COLUMN IF NOT EXISTS icon TEXT;
-- +goose StatementEnd

COMMENT ON TABLE js_plugin_versions IS 'JS异步任务插件-版本表（源码+编译产物+manifest）';
COMMENT ON COLUMN js_plugin_versions.id IS '主键';
COMMENT ON COLUMN js_plugin_versions.plugin_key IS '插件标识（meta.key）';
COMMENT ON COLUMN js_plugin_versions.version IS '插件版本（semver，key+version 唯一）';
COMMENT ON COLUMN js_plugin_versions.source IS 'plugin.js 源码原文（上传/内置 embed）';
COMMENT ON COLUMN js_plugin_versions.source_hash IS '源码 SHA-256（同版本不可改内容的校验依据）';
COMMENT ON COLUMN js_plugin_versions.compiled IS 'esbuild IIFE 编译产物（激活时编译缓存，可空）';
COMMENT ON COLUMN js_plugin_versions.manifest IS '解析校验后的 meta 清单';
COMMENT ON COLUMN js_plugin_versions.origin IS '版本来源：builtin=内置（go:embed 随二进制，不可删除）/ upload=管理端上传';
COMMENT ON COLUMN js_plugin_versions.icon IS '插件图标（data URI，仅上传版本；内置版本用 manifest.icon）';
COMMENT ON COLUMN js_plugin_versions.enabled IS '插件级开关（active 版本行上生效）';
COMMENT ON COLUMN js_plugin_versions.active IS '激活版本（每 plugin_key 至多一行）';
COMMENT ON COLUMN js_plugin_versions.remark IS '管理端备注';
COMMENT ON COLUMN js_plugin_versions.created_at IS '创建时间';
COMMENT ON COLUMN js_plugin_versions.updated_at IS '更新时间';

-- ── 2. 产物转存记录表 ──────────────────────────────────────
-- 插件在 parseQueryResponse 终态声明产物（key + taskData 取值路径），
-- 宿主流水线下载/解码 → FileService 上传对象存储 → 改写 task.Data 中的 URL。
-- 本表记录转存状态供重试（sweep 周期扫 pending/failed）与管理端观测；
-- 行内带齐任务归属（tenant/user/channel），进程重启后 sweep 重建 job 不丢归属。

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS js_task_artifacts (
    id             BIGSERIAL PRIMARY KEY,
    task_id        BIGINT        NOT NULL,
    tenant_id      BIGINT        NOT NULL DEFAULT 0,
    user_id        BIGINT        NOT NULL DEFAULT 0,
    channel_id     BIGINT        NOT NULL DEFAULT 0,
    public_task_id VARCHAR(64)   NOT NULL,
    plugin_key     VARCHAR(64)   NOT NULL,
    artifact_key   VARCHAR(128)  NOT NULL,
    source_type    VARCHAR(16)   NOT NULL,
    source_path    TEXT          NOT NULL DEFAULT '',
    source_auth    VARCHAR(32)   NOT NULL DEFAULT '',
    source_url     TEXT,
    stored_key     TEXT,
    backend        VARCHAR(32),
    mime_type      VARCHAR(128),
    size           BIGINT,
    status         VARCHAR(16)   NOT NULL DEFAULT 'pending',
    retry_count    INT           NOT NULL DEFAULT 0,
    last_error     TEXT,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (task_id, artifact_key)
);

CREATE INDEX IF NOT EXISTS idx_js_task_artifacts_status ON js_task_artifacts (status) WHERE status IN ('pending', 'failed');
CREATE INDEX IF NOT EXISTS idx_js_task_artifacts_tenant ON js_task_artifacts (tenant_id);

-- 幂等守卫：旧版形态表（列缺失）补列，列已存在为无操作
ALTER TABLE js_task_artifacts ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE js_task_artifacts ADD COLUMN IF NOT EXISTS user_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE js_task_artifacts ADD COLUMN IF NOT EXISTS channel_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE js_task_artifacts ADD COLUMN IF NOT EXISTS source_path TEXT NOT NULL DEFAULT '';
ALTER TABLE js_task_artifacts ADD COLUMN IF NOT EXISTS source_auth VARCHAR(32) NOT NULL DEFAULT '';
-- +goose StatementEnd

COMMENT ON TABLE js_task_artifacts IS 'JS任务插件-产物转存记录（声明→下载→对象存储→URL改写）';
COMMENT ON COLUMN js_task_artifacts.id IS '主键';
COMMENT ON COLUMN js_task_artifacts.task_id IS '关联任务 ID（tsk_model_tasks.id，逻辑关联无外键）';
COMMENT ON COLUMN js_task_artifacts.tenant_id IS '任务归属租户 ID（声明时随任务落库，sweep 恢复与租户观测用）';
COMMENT ON COLUMN js_task_artifacts.user_id IS '任务归属用户 ID（fil_files 落库归属）';
COMMENT ON COLUMN js_task_artifacts.channel_id IS '任务转发渠道 ID（相对 URL 产物按此解析渠道地址与鉴权）';
COMMENT ON COLUMN js_task_artifacts.public_task_id IS '公开任务 ID（task_xxx，存储 key 组成部分）';
COMMENT ON COLUMN js_task_artifacts.plugin_key IS '声明产物的 JS 插件 key';
COMMENT ON COLUMN js_task_artifacts.artifact_key IS '产物键（任务内唯一，如 video / audio-0）';
COMMENT ON COLUMN js_task_artifacts.source_type IS '产物来源类型：url=下载转存 / inline=base64 直存';
COMMENT ON COLUMN js_task_artifacts.source_path IS '产物声明的 taskData 取值路径（声明期写入，不随转存改写；回源代理与 sweep 重建的依据）';
COMMENT ON COLUMN js_task_artifacts.source_auth IS '产物声明的下载鉴权形态：空=裸下载 / channel-bearer=注入渠道密钥';
COMMENT ON COLUMN js_task_artifacts.source_url IS '实际下载 URL（url 型转存时写入；存量行声明期曾存取值路径）';
COMMENT ON COLUMN js_task_artifacts.stored_key IS '对象存储 key（jsartifacts/{public_task_id}/{artifact_key}{ext}，stored 后非空）';
COMMENT ON COLUMN js_task_artifacts.backend IS '对象存储后端：s3/oss/cos/local';
COMMENT ON COLUMN js_task_artifacts.mime_type IS '存储内容类型';
COMMENT ON COLUMN js_task_artifacts.size IS '转存字节数';
COMMENT ON COLUMN js_task_artifacts.status IS 'pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试） direct=不转存（插件关闭转存，回源代理）';
COMMENT ON COLUMN js_task_artifacts.retry_count IS '已重试次数（<5 由 sweep 重试）';
COMMENT ON COLUMN js_task_artifacts.last_error IS '最近一次失败原因';
COMMENT ON COLUMN js_task_artifacts.created_at IS '创建时间';
COMMENT ON COLUMN js_task_artifacts.updated_at IS '更新时间';

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS js_task_artifacts;
DROP TABLE IF EXISTS js_plugin_versions;
-- +goose StatementEnd
