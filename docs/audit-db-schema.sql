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
    task_completed_at                        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_aud_request_logs_created_brin
    ON aud_request_logs USING brin (created_at);
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_tenant
    ON aud_request_logs USING btree (tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_project
    ON aud_request_logs USING btree (project_id) WHERE project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_aud_request_logs_task_id
    ON aud_request_logs (task_id) WHERE task_id IS NOT NULL;

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
