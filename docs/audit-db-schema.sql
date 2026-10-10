-- 审计日志独立数据库建表脚本
--
-- 用途：将大模型请求审计日志（aud_request_logs）存储到独立的 PostgreSQL 实例，
--       与主业务库物理隔离，避免审计写入影响主库响应速度。
--
-- 使用方法：
--   1. 在 PostgreSQL 中创建独立数据库（如 team-api-audit）
--   2. 连接该数据库执行本脚本：
--      psql -h host -p port -U user -d team-api-audit -f docs/audit-db-schema.sql
--   3. 在 config.yaml 中配置 database.audit 连接信息
--   4. 重启服务
--
-- 注意：仅 aud_request_logs 表走独立库，其余审计表（aud_operation_logs、
--       aud_login_history、aud_sensitive_access_logs、aud_content_filter_logs）
--       仍留在主库，无需迁移。

CREATE TABLE IF NOT EXISTS aud_request_logs (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    api_key_id                               BIGINT,
    request_id                               VARCHAR(64) NOT NULL,
    method                                   VARCHAR(10) NOT NULL,
    path                                     VARCHAR(500) NOT NULL,
    query_params                             TEXT,
    status_code                              INTEGER,
    client_ip                                VARCHAR(45),
    user_agent                               TEXT,
    request_body                             TEXT,
    response_body                            TEXT,
    latency_ms                               INTEGER,
    audit_level                              VARCHAR(20) DEFAULT 'full' NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    tenant_request_body                      TEXT,
    tenant_response_body                     TEXT,
    tenant_audit_level                       VARCHAR(20) DEFAULT 'full' NOT NULL,
    project_id                               BIGINT,
    first_token_ms                           INTEGER,
    request_headers                          JSONB,
    response_headers                         JSONB,
    forwarding_trace                         JSONB,
    task_id                                  VARCHAR(64),
    task_status                              VARCHAR(20),
    task_result                              TEXT,
    task_upstream_headers                    JSONB,
    task_completed_at                        TIMESTAMPTZ,
    model_name                               VARCHAR(100),
    session_key                              VARCHAR(128),
    session_source                           VARCHAR(20),
    conv_fingerprint                         VARCHAR(80),
    resp_msg_hash                            VARCHAR(80),
    last_asst_msg_hash                       VARCHAR(80),
    is_context_reset                         BOOLEAN DEFAULT FALSE NOT NULL,
    user_field                               VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_aud_request_logs_created_brin
    ON aud_request_logs USING brin (created_at);
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_tenant
    ON aud_request_logs USING btree (tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_project
    ON aud_request_logs USING btree (project_id) WHERE project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_task_id
    ON aud_request_logs (task_id) WHERE task_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_model
    ON aud_request_logs (model_name, created_at);
-- created_at 单列 btree（对应迁移 000024）：服务无租户条件的全局倒序分页
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_created_bt
    ON aud_request_logs USING btree (created_at);
-- 会话关联（对应迁移 000025）：会话视图按租户过滤后按键取时间有序记录；
-- resp_hash 为衔接链前驱点查。列均允许 NULL（无信号/非对话端点不落值）
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_session
    ON aud_request_logs USING btree (tenant_id, session_key, created_at) WHERE session_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_conv_fp
    ON aud_request_logs USING btree (tenant_id, conv_fingerprint, created_at) WHERE conv_fingerprint IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_resp_hash
    ON aud_request_logs USING btree (tenant_id, resp_msg_hash) WHERE resp_msg_hash IS NOT NULL;

COMMENT ON TABLE  aud_request_logs IS '请求审计日志（记录所有 API 代理请求）';
COMMENT ON COLUMN aud_request_logs.id IS '主键ID';
COMMENT ON COLUMN aud_request_logs.tenant_id IS '租户ID';
COMMENT ON COLUMN aud_request_logs.user_id IS '用户ID';
COMMENT ON COLUMN aud_request_logs.api_key_id IS '使用的 API Key ID';
COMMENT ON COLUMN aud_request_logs.request_id IS '请求唯一ID（关联全链路追踪）';
COMMENT ON COLUMN aud_request_logs.method IS 'HTTP 方法（GET/POST/PUT/DELETE）';
COMMENT ON COLUMN aud_request_logs.path IS '请求路径';
COMMENT ON COLUMN aud_request_logs.query_params IS '查询参数（URL Query String）';
COMMENT ON COLUMN aud_request_logs.status_code IS 'HTTP 响应状态码';
COMMENT ON COLUMN aud_request_logs.client_ip IS '客户端 IP';
COMMENT ON COLUMN aud_request_logs.user_agent IS '客户端 User-Agent';
COMMENT ON COLUMN aud_request_logs.request_body IS '请求体（脱敏后存储）';
COMMENT ON COLUMN aud_request_logs.response_body IS '响应体（截断后存储）';
COMMENT ON COLUMN aud_request_logs.latency_ms IS '请求延迟（毫秒）';
COMMENT ON COLUMN aud_request_logs.audit_level IS '审计级别：full（完整记录）/ full_text（全量文本）/ masked（脱敏记录）/ question_only（仅提问）/ none（不记录）';
COMMENT ON COLUMN aud_request_logs.created_at IS '创建时间';
COMMENT ON COLUMN aud_request_logs.updated_at IS '更新时间';
COMMENT ON COLUMN aud_request_logs.tenant_request_body IS '租户级请求体（按租户审计级别处理）';
COMMENT ON COLUMN aud_request_logs.tenant_response_body IS '租户级响应体（按租户审计级别处理）';
COMMENT ON COLUMN aud_request_logs.tenant_audit_level IS '租户审计级别：full / full_text / masked / question_only / none';
COMMENT ON COLUMN aud_request_logs.project_id IS '关联项目ID（通过API Key关联，NULL表示个人密钥无项目）';
COMMENT ON COLUMN aud_request_logs.first_token_ms IS '首个 Token 出现的用时（毫秒），仅流式请求有值';
COMMENT ON COLUMN aud_request_logs.request_headers IS '请求头信息（仅审计级别为 full 时记录，管理后台调试用）';
COMMENT ON COLUMN aud_request_logs.response_headers IS '响应头信息（仅审计级别为 full 时记录，管理后台调试用）';
COMMENT ON COLUMN aud_request_logs.forwarding_trace IS '请求转发路径追踪（仅管理员可见）';
COMMENT ON COLUMN aud_request_logs.task_id IS '异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id';
COMMENT ON COLUMN aud_request_logs.task_status IS '异步任务终态：SUCCESS / FAILURE';
COMMENT ON COLUMN aud_request_logs.task_result IS '异步任务完成时上游返回的原始响应体';
COMMENT ON COLUMN aud_request_logs.task_upstream_headers IS '异步任务完成时上游返回的响应头（仅审计级别为 full 时记录）';
COMMENT ON COLUMN aud_request_logs.task_completed_at IS '异步任务达到终态的时间';
COMMENT ON COLUMN aud_request_logs.model_name IS '请求使用的模型名（用户请求体中的原始模型，未经渠道映射）';
COMMENT ON COLUMN aud_request_logs.session_key IS '会话关联键（sk:来源:sha256），仅显式会话信号（X-Session-Id 头/Claude metadata.user_id/Responses 链）命中时写入，NULL=无显式键';
COMMENT ON COLUMN aud_request_logs.session_source IS '会话键来源：header/anthropic/openai，NULL=无显式键';
COMMENT ON COLUMN aud_request_logs.conv_fingerprint IS '会话指纹 fp1:sha256（system+首条user消息规范化哈希），同会话各轮共享，内容分组用';
COMMENT ON COLUMN aud_request_logs.resp_msg_hash IS '本条响应 assistant 消息规范化哈希 mh1:sha256，衔接链前驱查询用';
COMMENT ON COLUMN aud_request_logs.last_asst_msg_hash IS '本条请求 messages 最后一条 assistant 消息规范化哈希 mh1:sha256，与上一轮 resp_msg_hash 相等即两轮衔接';
COMMENT ON COLUMN aud_request_logs.is_context_reset IS '写时检测到上下文压缩/重写标记（best-effort，权威判定在会话重组查询时按消息量骤降判定）';
COMMENT ON COLUMN aud_request_logs.user_field IS '请求体 user 字段原文（OpenAI 协议 hint，不参与会话键解析）';

-- ============================================================
-- 增量变更（存量审计库升级用）
--
-- 本脚本整体幂等（IF NOT EXISTS），存量审计库升级时直接重跑全脚本即可；
-- 也可只执行下方对应版本的增量语句。主库的同等变更由 goose 迁移
-- migrations/000022、000024、000025 管理，两边需保持同步。
-- ============================================================

-- 000022：补记模型名（存量行无法回填，模型名自升级后开始记录）
ALTER TABLE aud_request_logs ADD COLUMN IF NOT EXISTS model_name VARCHAR(100);
COMMENT ON COLUMN aud_request_logs.model_name IS '请求使用的模型名（用户请求体中的原始模型，未经渠道映射）';
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_model
    ON aud_request_logs (model_name, created_at);
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_created_bt
    ON aud_request_logs USING btree (created_at);

-- 000025：会话关联列（哈希均在原始请求/响应体上计算，先于审计级别处理与截断，
-- 关联能力不受审计级别影响；设计见 docs/会话关联与完整会话提取方案.md）
ALTER TABLE aud_request_logs ADD COLUMN IF NOT EXISTS session_key VARCHAR(128);
ALTER TABLE aud_request_logs ADD COLUMN IF NOT EXISTS session_source VARCHAR(20);
ALTER TABLE aud_request_logs ADD COLUMN IF NOT EXISTS conv_fingerprint VARCHAR(80);
ALTER TABLE aud_request_logs ADD COLUMN IF NOT EXISTS resp_msg_hash VARCHAR(80);
ALTER TABLE aud_request_logs ADD COLUMN IF NOT EXISTS last_asst_msg_hash VARCHAR(80);
ALTER TABLE aud_request_logs ADD COLUMN IF NOT EXISTS is_context_reset BOOLEAN DEFAULT FALSE NOT NULL;
ALTER TABLE aud_request_logs ADD COLUMN IF NOT EXISTS user_field VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_session
    ON aud_request_logs USING btree (tenant_id, session_key, created_at) WHERE session_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_conv_fp
    ON aud_request_logs USING btree (tenant_id, conv_fingerprint, created_at) WHERE conv_fingerprint IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_resp_hash
    ON aud_request_logs USING btree (tenant_id, resp_msg_hash) WHERE resp_msg_hash IS NOT NULL;
