-- +goose Up
-- 租户模型自定义定价 + 异步任务表重命名 + 内容过滤日志表

-- 1. 租户模型分配增加自定义缓存定价覆盖字段和阶梯定价支持
ALTER TABLE mdl_tenant_models
    ADD COLUMN IF NOT EXISTS custom_cache_read_price     NUMERIC(20,10),
    ADD COLUMN IF NOT EXISTS custom_cache_creation_price  NUMERIC(20,10);

COMMENT ON COLUMN mdl_tenant_models.custom_cache_read_price IS '自定义缓存读取价格（$/1M token），NULL 表示使用基础定价';
COMMENT ON COLUMN mdl_tenant_models.custom_cache_creation_price IS '自定义缓存创建价格（$/1M token），NULL 表示使用基础定价';

ALTER TABLE mdl_tenant_models
    ADD COLUMN IF NOT EXISTS custom_pricing_tiers JSONB;

COMMENT ON COLUMN mdl_tenant_models.custom_pricing_tiers IS '自定义阶梯定价（JSONB 数组），格式: [{"min_tokens":0,"max_tokens":100000,"input_price":0.5,"output_price":1.5,"cache_read_price":0.1,"cache_creation_price":0.2}]';

-- 2. 将 tsk_async_tasks 重命名为 tsk_model_tasks，并添加表和字段注释
ALTER TABLE tsk_async_tasks RENAME TO tsk_model_tasks;

ALTER TABLE tsk_model_tasks RENAME CONSTRAINT uk_tsk_async_tasks_public_id TO uk_tsk_model_tasks_public_id;

ALTER INDEX idx_tsk_async_tasks_active RENAME TO idx_tsk_model_tasks_active;
ALTER INDEX idx_tsk_async_tasks_status RENAME TO idx_tsk_model_tasks_status;
ALTER INDEX idx_tsk_async_tasks_submit_time RENAME TO idx_tsk_model_tasks_submit_time;
ALTER INDEX idx_tsk_async_tasks_user RENAME TO idx_tsk_model_tasks_user;

COMMENT ON TABLE tsk_model_tasks IS '异步模型任务表，存储 AI 模型异步调用任务（如视频生成、图像生成、音乐生成等）的状态和结果';
COMMENT ON COLUMN tsk_model_tasks.id IS '主键ID';
COMMENT ON COLUMN tsk_model_tasks.public_task_id IS '对外公开的任务ID（task_xxxxx 格式），用于 API 响应和客户端查询';
COMMENT ON COLUMN tsk_model_tasks.platform IS '任务所属平台：sora、kling、suno、midjourney、volcengine';
COMMENT ON COLUMN tsk_model_tasks.action IS '任务动作类型。通用：generate；Suno：music、lyrics；';
COMMENT ON COLUMN tsk_model_tasks.status IS '任务状态：NOT_START-未开始、SUBMITTED-已提交、IN_PROGRESS-进行中、SUCCESS-成功、FAILURE-失败';
COMMENT ON COLUMN tsk_model_tasks.progress IS '任务进度，字符串格式百分比（如 0%、50%、100%）';
COMMENT ON COLUMN tsk_model_tasks.fail_reason IS '任务失败原因文本';
COMMENT ON COLUMN tsk_model_tasks.tenant_id IS '发起任务的租户ID';
COMMENT ON COLUMN tsk_model_tasks.user_id IS '发起任务的租户用户ID';
COMMENT ON COLUMN tsk_model_tasks.api_key_id IS '调用方使用的 API Key ID';
COMMENT ON COLUMN tsk_model_tasks.channel_id IS '转发请求的渠道ID';
COMMENT ON COLUMN tsk_model_tasks.model_name IS '用户请求的模型名称（如 sora-1.0-turbo、midjourney 等）';
COMMENT ON COLUMN tsk_model_tasks.upstream_model IS '上游供应商实际使用的模型名称，可能与请求模型不同';
COMMENT ON COLUMN tsk_model_tasks.pre_deduct_amount IS '提交任务时的预扣金额（USD），任务完成后根据实际用量结算差额';
COMMENT ON COLUMN tsk_model_tasks.actual_cost IS '任务实际消费金额（USD），成功时按此结算，失败时退还预扣金额';
COMMENT ON COLUMN tsk_model_tasks.billing_settled IS '计费是否已结算（true 表示已完成预扣与实际金额的多退少补）';
COMMENT ON COLUMN tsk_model_tasks.result_url IS '任务完成后的结果资源 URL（如生成的视频/图片/音频的下载地址）';
COMMENT ON COLUMN tsk_model_tasks.data IS '上游供应商返回的完整响应数据（JSONB），可返回给用户查看';
COMMENT ON COLUMN tsk_model_tasks.private_data IS '内部私有数据（JSONB），含 upstream_task_id 等敏感字段，用于轮询上游状态，不返回给用户';
COMMENT ON COLUMN tsk_model_tasks.submit_time IS '任务提交到上游供应商的时间';
COMMENT ON COLUMN tsk_model_tasks.start_time IS '上游供应商开始执行任务的时间';
COMMENT ON COLUMN tsk_model_tasks.finish_time IS '任务完成（成功或失败）的时间';
COMMENT ON COLUMN tsk_model_tasks.created_at IS '记录创建时间';
COMMENT ON COLUMN tsk_model_tasks.updated_at IS '记录更新时间';

-- 3. 创建内容过滤拦截日志表
CREATE TABLE aud_content_filter_logs (
    id                 BIGSERIAL PRIMARY KEY,
    tenant_id          BIGINT,
    user_id            BIGINT,
    api_key_id         BIGINT,
    project_id         BIGINT,
    request_id         VARCHAR(64),
    method             VARCHAR(10),
    path               VARCHAR(500),
    client_ip          VARCHAR(45),
    filter_mode        VARCHAR(20) NOT NULL,
    matched_words      JSONB NOT NULL,
    original_snippet   TEXT,
    blocked            BOOLEAN NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE INDEX idx_aud_content_filter_logs_created_brin ON aud_content_filter_logs USING brin (created_at);
CREATE INDEX idx_aud_content_filter_logs_tenant ON aud_content_filter_logs USING btree (tenant_id, created_at);
CREATE INDEX idx_aud_content_filter_logs_mode ON aud_content_filter_logs (filter_mode);

COMMENT ON TABLE aud_content_filter_logs IS '内容过滤拦截日志，记录所有命中敏感词的请求';
COMMENT ON COLUMN aud_content_filter_logs.id IS '主键ID';
COMMENT ON COLUMN aud_content_filter_logs.tenant_id IS '租户ID';
COMMENT ON COLUMN aud_content_filter_logs.user_id IS '用户ID';
COMMENT ON COLUMN aud_content_filter_logs.api_key_id IS 'API Key ID';
COMMENT ON COLUMN aud_content_filter_logs.project_id IS '项目ID';
COMMENT ON COLUMN aud_content_filter_logs.request_id IS '请求唯一ID';
COMMENT ON COLUMN aud_content_filter_logs.method IS 'HTTP 方法';
COMMENT ON COLUMN aud_content_filter_logs.path IS '请求路径';
COMMENT ON COLUMN aud_content_filter_logs.client_ip IS '客户端 IP';
COMMENT ON COLUMN aud_content_filter_logs.filter_mode IS '过滤模式：log / replace / block';
COMMENT ON COLUMN aud_content_filter_logs.matched_words IS '命中的敏感词列表（JSONB 数组）';
COMMENT ON COLUMN aud_content_filter_logs.original_snippet IS '原始请求体片段（截断存储，仅 replace 模式）';
COMMENT ON COLUMN aud_content_filter_logs.blocked IS '是否被拦截（mode=block 时为 true）';
COMMENT ON COLUMN aud_content_filter_logs.created_at IS '创建时间';

-- +goose Down
-- 3. 删除内容过滤日志表
DROP TABLE IF EXISTS aud_content_filter_logs;

-- 2. 移除注释
COMMENT ON TABLE tsk_model_tasks IS NULL;
COMMENT ON COLUMN tsk_model_tasks.id IS NULL;
COMMENT ON COLUMN tsk_model_tasks.public_task_id IS NULL;
COMMENT ON COLUMN tsk_model_tasks.platform IS NULL;
COMMENT ON COLUMN tsk_model_tasks.action IS NULL;
COMMENT ON COLUMN tsk_model_tasks.status IS NULL;
COMMENT ON COLUMN tsk_model_tasks.progress IS NULL;
COMMENT ON COLUMN tsk_model_tasks.fail_reason IS NULL;
COMMENT ON COLUMN tsk_model_tasks.tenant_id IS NULL;
COMMENT ON COLUMN tsk_model_tasks.user_id IS NULL;
COMMENT ON COLUMN tsk_model_tasks.api_key_id IS NULL;
COMMENT ON COLUMN tsk_model_tasks.channel_id IS NULL;
COMMENT ON COLUMN tsk_model_tasks.model_name IS NULL;
COMMENT ON COLUMN tsk_model_tasks.upstream_model IS NULL;
COMMENT ON COLUMN tsk_model_tasks.pre_deduct_amount IS NULL;
COMMENT ON COLUMN tsk_model_tasks.actual_cost IS NULL;
COMMENT ON COLUMN tsk_model_tasks.billing_settled IS NULL;
COMMENT ON COLUMN tsk_model_tasks.result_url IS NULL;
COMMENT ON COLUMN tsk_model_tasks.data IS NULL;
COMMENT ON COLUMN tsk_model_tasks.private_data IS NULL;
COMMENT ON COLUMN tsk_model_tasks.submit_time IS NULL;
COMMENT ON COLUMN tsk_model_tasks.start_time IS NULL;
COMMENT ON COLUMN tsk_model_tasks.finish_time IS NULL;
COMMENT ON COLUMN tsk_model_tasks.created_at IS NULL;
COMMENT ON COLUMN tsk_model_tasks.updated_at IS NULL;

-- 恢复索引名
ALTER INDEX idx_tsk_model_tasks_user RENAME TO idx_tsk_async_tasks_user;
ALTER INDEX idx_tsk_model_tasks_submit_time RENAME TO idx_tsk_async_tasks_submit_time;
ALTER INDEX idx_tsk_model_tasks_status RENAME TO idx_tsk_async_tasks_status;
ALTER INDEX idx_tsk_model_tasks_active RENAME TO idx_tsk_async_tasks_active;

-- 恢复约束名
ALTER TABLE tsk_model_tasks RENAME CONSTRAINT uk_tsk_model_tasks_public_id TO uk_tsk_async_tasks_public_id;

-- 恢复表名
ALTER TABLE tsk_model_tasks RENAME TO tsk_async_tasks;

-- 1. 移除定价字段
ALTER TABLE mdl_tenant_models
    DROP COLUMN IF EXISTS custom_cache_read_price,
    DROP COLUMN IF EXISTS custom_cache_creation_price,
    DROP COLUMN IF EXISTS custom_pricing_tiers;
