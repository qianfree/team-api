-- +goose Up
-- 异步任务支持、计费类型优化、交易关联字段、预扣追踪表

-- ============================================================
-- 1. 异步任务支持（原 000003）
-- ============================================================

-- 1.1 aud_request_logs 新增任务结果相关字段
ALTER TABLE aud_request_logs
    ADD COLUMN IF NOT EXISTS task_id               VARCHAR(64),
    ADD COLUMN IF NOT EXISTS task_status           VARCHAR(20),
    ADD COLUMN IF NOT EXISTS task_result           TEXT,
    ADD COLUMN IF NOT EXISTS task_upstream_headers JSONB,
    ADD COLUMN IF NOT EXISTS task_completed_at     TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_aud_request_logs_task_id
    ON aud_request_logs (task_id) WHERE task_id IS NOT NULL;

COMMENT ON COLUMN aud_request_logs.task_id IS '异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id';
COMMENT ON COLUMN aud_request_logs.task_status IS '异步任务终态：SUCCESS / FAILURE';
COMMENT ON COLUMN aud_request_logs.task_result IS '异步任务完成时上游返回的原始响应体';
COMMENT ON COLUMN aud_request_logs.task_upstream_headers IS '异步任务完成时上游返回的响应头（仅审计级别为 full 时记录）';
COMMENT ON COLUMN aud_request_logs.task_completed_at IS '异步任务达到终态的时间';

-- 1.2 bil_usage_logs 新增 task_id 字段
ALTER TABLE bil_usage_logs
    ADD COLUMN IF NOT EXISTS task_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_bil_usage_logs_task_id
    ON bil_usage_logs (task_id) WHERE task_id IS NOT NULL;

COMMENT ON COLUMN bil_usage_logs.task_id IS '异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id，普通请求为空';

-- 1.3 bil_usage_logs.request_type 注释更新（新增 async=3）
COMMENT ON COLUMN bil_usage_logs.request_type IS '请求类型: 1=sync, 2=stream, 3=async, 4=websocket';

-- 1.4 tsk_model_tasks 新增 request_id 字段
ALTER TABLE tsk_model_tasks
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_tsk_model_tasks_request_id
    ON tsk_model_tasks (request_id) WHERE request_id IS NOT NULL;

COMMENT ON COLUMN tsk_model_tasks.request_id IS '任务提交时的原始请求 ID（req_xxxxx），关联 aud_request_logs.request_id';

-- ============================================================
-- 2. 计费类型优化
-- ============================================================
COMMENT ON COLUMN bil_transactions.type IS '类型：consume（消费）/ recharge（充值）/ adjust（调整）/ pre_deduct（预扣，已废弃）/ settle（结算，已废弃）/ refund（退款，已废弃）/ freeze（冻结，已废弃）/ unfreeze（解冻，已废弃）';

-- ============================================================
-- 3. bil_transactions 关联字段
-- ============================================================
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS user_id BIGINT;
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS request_id VARCHAR(64);
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS model_name VARCHAR(100);

COMMENT ON COLUMN bil_transactions.user_id IS '关联用户ID（consume 类型为实际消费用户，recharge 类型为操作用户，adjust 类型为空）';
COMMENT ON COLUMN bil_transactions.request_id IS '关联请求ID（consume 类型对应 API 调用的 request_id，其他类型为空）';
COMMENT ON COLUMN bil_transactions.model_name IS '关联模型名（consume 类型为调用的模型名，其他类型为空）';

CREATE INDEX IF NOT EXISTS idx_bil_transactions_user_created ON bil_transactions (tenant_id, user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bil_transactions_model_created ON bil_transactions (tenant_id, model_name, created_at DESC) WHERE model_name IS NOT NULL;

-- ============================================================
-- 4. 删除 ord_payment_channels 表
-- ============================================================
DROP TABLE IF EXISTS ord_payment_channels;

-- ============================================================
-- 5. bil_transactions 项目/任务关联字段
-- ============================================================
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS project_id BIGINT;
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS api_key_id BIGINT;
ALTER TABLE bil_transactions ADD COLUMN IF NOT EXISTS task_id VARCHAR(64);

COMMENT ON COLUMN bil_transactions.project_id IS '关联项目ID（consume 类型为 API Key 所属项目，个人密钥为空）';
COMMENT ON COLUMN bil_transactions.api_key_id IS '关联API密钥ID（consume 类型为发起请求的密钥）';
COMMENT ON COLUMN bil_transactions.task_id IS '关联异步任务公开ID（consume+relay_mode=task 时关联 tsk_model_tasks.public_task_id）';

CREATE INDEX IF NOT EXISTS idx_bil_transactions_project_created ON bil_transactions (tenant_id, project_id, created_at DESC) WHERE project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_bil_transactions_apikey_created ON bil_transactions (tenant_id, api_key_id, created_at DESC) WHERE api_key_id IS NOT NULL;

-- ============================================================
-- 6. 预扣追踪表
-- ============================================================
CREATE TABLE bil_prededuct_tracks (
    id           BIGSERIAL PRIMARY KEY,
    tenant_id    BIGINT          NOT NULL,
    request_id   VARCHAR(64)     NOT NULL,
    amount       NUMERIC(20,10)  NOT NULL,
    model_name   VARCHAR(100),
    status       VARCHAR(20)     NOT NULL DEFAULT 'frozen',
    created_at   TIMESTAMPTZ     NOT NULL DEFAULT now(),
    expired_at   TIMESTAMPTZ,
    CONSTRAINT uk_prededuct_request UNIQUE (request_id),
    CONSTRAINT chk_prededuct_status CHECK (status IN ('frozen','settled','expired','released'))
);

CREATE INDEX idx_prededuct_tracks_cleanup ON bil_prededuct_tracks (status, created_at)
    WHERE status = 'frozen';
CREATE INDEX idx_prededuct_tracks_tenant ON bil_prededuct_tracks (tenant_id, created_at DESC);

COMMENT ON TABLE bil_prededuct_tracks IS '预扣追踪表，记录每个预扣的生命周期用于孤儿清理';
COMMENT ON COLUMN bil_prededuct_tracks.tenant_id IS '租户 ID';
COMMENT ON COLUMN bil_prededuct_tracks.request_id IS '请求唯一 ID';
COMMENT ON COLUMN bil_prededuct_tracks.amount IS '预扣金额（USD）';
COMMENT ON COLUMN bil_prededuct_tracks.model_name IS '模型名称';
COMMENT ON COLUMN bil_prededuct_tracks.status IS 'frozen=冻结中, settled=已结算, expired=超时自动释放, released=手动释放';
COMMENT ON COLUMN bil_prededuct_tracks.created_at IS '创建时间';
COMMENT ON COLUMN bil_prededuct_tracks.expired_at IS '过期释放时间（仅 status=expired 时有值）';

-- +goose Down
-- 按 8→3 逆序回滚

-- 6. 预扣追踪表
DROP TABLE IF EXISTS bil_prededuct_tracks;

-- 5. bil_transactions 项目/任务关联字段
DROP INDEX IF EXISTS idx_bil_transactions_apikey_created;
DROP INDEX IF EXISTS idx_bil_transactions_project_created;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS task_id;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS api_key_id;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS project_id;

-- 4. 恢复 ord_payment_channels 表
CREATE TABLE ord_payment_channels (
    id          BIGSERIAL PRIMARY KEY,
    channel     VARCHAR(20) NOT NULL,
    name        VARCHAR(100) NOT NULL,
    config      JSONB DEFAULT '{}' NOT NULL,
    is_enabled  BOOLEAN DEFAULT false NOT NULL,
    sort_order  INTEGER DEFAULT 0 NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now(),
    payment_type VARCHAR(20) DEFAULT '' NOT NULL,
    callback_url VARCHAR(500) DEFAULT '' NOT NULL,
    return_url   VARCHAR(500) DEFAULT '' NOT NULL
);
CREATE INDEX idx_ord_payment_channels_channel ON ord_payment_channels USING btree (channel);
COMMENT ON TABLE ord_payment_channels IS '支付渠道配置';
COMMENT ON COLUMN ord_payment_channels.id IS '主键ID';
COMMENT ON COLUMN ord_payment_channels.channel IS '渠道标识（alipay/wechat/stripe/mock）';
COMMENT ON COLUMN ord_payment_channels.name IS '显示名称';
COMMENT ON COLUMN ord_payment_channels.config IS '渠道配置（JSONB，含 API 密钥等敏感信息）';
COMMENT ON COLUMN ord_payment_channels.is_enabled IS '是否启用';
COMMENT ON COLUMN ord_payment_channels.sort_order IS '排序权重';
COMMENT ON COLUMN ord_payment_channels.created_at IS '创建时间';
COMMENT ON COLUMN ord_payment_channels.updated_at IS '更新时间';
COMMENT ON COLUMN ord_payment_channels.payment_type IS '子支付方式（alipay/wxpay 等，空表示该渠道支持所有方式）';
COMMENT ON COLUMN ord_payment_channels.callback_url IS '支付回调地址覆盖（为空则使用系统默认）';
COMMENT ON COLUMN ord_payment_channels.return_url IS '支付完成后前端跳转地址覆盖';

-- 3. bil_transactions 关联字段
DROP INDEX IF EXISTS idx_bil_transactions_model_created;
DROP INDEX IF EXISTS idx_bil_transactions_user_created;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS model_name;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS request_id;
ALTER TABLE bil_transactions DROP COLUMN IF EXISTS user_id;

-- 2. 计费类型注释回滚
COMMENT ON COLUMN bil_transactions.type IS '类型：recharge（充值）/ pre_deduct（预扣）/ settle（结算）/ refund（退款）/ adjust（调整）/ freeze（冻结）/ unfreeze（解冻）';

-- 1. 异步任务支持
DROP INDEX IF EXISTS idx_tsk_model_tasks_request_id;
ALTER TABLE tsk_model_tasks
    DROP COLUMN IF EXISTS request_id;

DROP INDEX IF EXISTS idx_aud_request_logs_task_id;
ALTER TABLE aud_request_logs
    DROP COLUMN IF EXISTS task_id,
    DROP COLUMN IF EXISTS task_status,
    DROP COLUMN IF EXISTS task_result,
    DROP COLUMN IF EXISTS task_upstream_headers,
    DROP COLUMN IF EXISTS task_completed_at;

DROP INDEX IF EXISTS idx_bil_usage_logs_task_id;
ALTER TABLE bil_usage_logs
    DROP COLUMN IF EXISTS task_id;

COMMENT ON COLUMN bil_usage_logs.request_type IS '请求类型: 1=sync, 2=stream, 3=websocket';
