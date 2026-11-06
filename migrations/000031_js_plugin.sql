-- +goose Up
-- ============================================================
-- JS 异步任务插件体系：数据表（原 000031~000034 四个迁移合并为一份）
--
--   · js_plugin_versions   插件版本表（源码 + 编译产物 + manifest + origin/icon）
--   · js_task_artifacts    产物转存记录表（声明 → 下载 → 对象存储 → URL 改写）
--
-- 幂等性（三重守卫，任何部署状态重跑均无操作）：
--   表不存在 → 新建（列定义齐全）；旧版表已存在 → ADD COLUMN IF NOT EXISTS 补列；
--   列已存在 → 跳过。设计见 docs/design/JS异步任务插件设计方案-v1.md §九/§十五-§十八。

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

-- 兼容旧版建表（列缺失时补加；列已存在为无操作）
ALTER TABLE js_plugin_versions ADD COLUMN IF NOT EXISTS origin VARCHAR(16) NOT NULL DEFAULT 'upload';
ALTER TABLE js_plugin_versions ADD COLUMN IF NOT EXISTS icon TEXT;
-- +goose StatementEnd

COMMENT ON TABLE js_plugin_versions IS 'JS异步任务插件-版本表（源码+编译产物+manifest）';
COMMENT ON COLUMN js_plugin_versions.plugin_key IS '插件标识（meta.key）';
COMMENT ON COLUMN js_plugin_versions.compiled IS 'esbuild IIFE 编译产物（激活时编译缓存，可空）';
COMMENT ON COLUMN js_plugin_versions.manifest IS '解析校验后的 meta 清单';
COMMENT ON COLUMN js_plugin_versions.origin IS '版本来源：builtin=内置（go:embed 随二进制，不可删除）/ upload=管理端上传';
COMMENT ON COLUMN js_plugin_versions.icon IS '插件图标（data URI，仅上传版本；内置版本用 manifest.icon）';
COMMENT ON COLUMN js_plugin_versions.enabled IS '插件级开关（active 版本行上生效）';
COMMENT ON COLUMN js_plugin_versions.active IS '激活版本（每 plugin_key 至多一行）';

-- ── 2. 产物转存记录表 ──────────────────────────────────────
-- 插件在 parseQueryResponse 终态声明产物（key + taskData 取值路径），宿主流水线
-- 下载/解码 → FileService 上传对象存储 → 改写 task.Data 中的 URL。本表记录
-- 转存状态供重试（sweep 周期扫 pending/failed）与管理端观测。

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS js_task_artifacts (
    id             BIGSERIAL PRIMARY KEY,
    task_id        BIGINT        NOT NULL,
    public_task_id VARCHAR(64)   NOT NULL,
    plugin_key     VARCHAR(64)   NOT NULL,
    artifact_key   VARCHAR(128)  NOT NULL,
    source_type    VARCHAR(16)   NOT NULL,               -- url | inline
    source_url     TEXT,                                 -- 上游原始 URL（url 型，回落用）
    stored_key     TEXT,                                 -- 对象存储 key（stored 后非空）
    backend        VARCHAR(32),                          -- s3/oss/cos/local
    mime_type      VARCHAR(128),
    size           BIGINT,
    status         VARCHAR(16)   NOT NULL DEFAULT 'pending', -- pending/running/stored/failed
    retry_count    INT           NOT NULL DEFAULT 0,
    last_error     TEXT,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (task_id, artifact_key)
);

CREATE INDEX IF NOT EXISTS idx_js_task_artifacts_status ON js_task_artifacts (status) WHERE status IN ('pending', 'failed');
-- +goose StatementEnd

COMMENT ON TABLE js_task_artifacts IS 'JS任务插件-产物转存记录（声明→下载→对象存储→URL改写）';
COMMENT ON COLUMN js_task_artifacts.status IS 'pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试）';

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS js_task_artifacts;
DROP TABLE IF EXISTS js_plugin_versions;
-- +goose StatementEnd
