-- +goose Up
-- 渠道上游错误事件表：记录每次渠道错误（含重试中间失败），用于管理后台错误监控和告警
CREATE TABLE chn_error_events (
    id               BIGSERIAL,
    channel_id       BIGINT          NOT NULL,
    channel_name     VARCHAR(100)    NOT NULL,
    channel_type     INT             NOT NULL,
    provider         VARCHAR(50)     NOT NULL,
    model_name       VARCHAR(100)    NOT NULL,
    upstream_model   VARCHAR(100),
    request_id       VARCHAR(64)     NOT NULL,
    tenant_id        BIGINT          NOT NULL,
    error_category   VARCHAR(30)     NOT NULL,
    status_code      INT             NOT NULL,
    error_type       VARCHAR(30)     NOT NULL,
    error_message    TEXT            NOT NULL,
    is_retryable     BOOLEAN         NOT NULL DEFAULT false,
    attempt          INT             NOT NULL DEFAULT 0,
    is_final         BOOLEAN         NOT NULL DEFAULT false,
    latency_ms       FLOAT8          DEFAULT 0,
    created_at       TIMESTAMPTZ     NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- BRIN 索引：追加写表的时间范围查询
CREATE INDEX idx_chn_error_events_created_brin ON chn_error_events USING BRIN (created_at);

-- 渠道 + 时间：最常用的筛选组合
CREATE INDEX idx_chn_error_events_channel_created ON chn_error_events (channel_id, created_at DESC);

-- 分类 + 时间：按错误类型统计
CREATE INDEX idx_chn_error_events_category_created ON chn_error_events (error_category, created_at DESC);

-- 请求 ID：关联追踪
CREATE INDEX idx_chn_error_events_request_id ON chn_error_events (request_id);

-- 字段注释
COMMENT ON TABLE chn_error_events IS '渠道上游错误事件表（追加写入，按月分区）';
COMMENT ON COLUMN chn_error_events.id IS '主键ID';
COMMENT ON COLUMN chn_error_events.channel_id IS '发生错误的渠道ID';
COMMENT ON COLUMN chn_error_events.channel_name IS '渠道名称（冗余存储，避免查询时JOIN）';
COMMENT ON COLUMN chn_error_events.channel_type IS '渠道类型（ProviderType枚举值）';
COMMENT ON COLUMN chn_error_events.provider IS '供应商名称（如 OpenAI, Claude, Zhipu 等）';
COMMENT ON COLUMN chn_error_events.model_name IS '请求的模型名';
COMMENT ON COLUMN chn_error_events.upstream_model IS '上游实际模型名（模型映射后）';
COMMENT ON COLUMN chn_error_events.request_id IS '关联的请求唯一ID';
COMMENT ON COLUMN chn_error_events.tenant_id IS '租户ID';
COMMENT ON COLUMN chn_error_events.error_category IS '错误分类：rate_limit/auth_error/timeout/upstream_error/server_error/network_error/unknown';
COMMENT ON COLUMN chn_error_events.status_code IS 'HTTP状态码（来自上游响应或RelayError.StatusCode）';
COMMENT ON COLUMN chn_error_events.error_type IS 'RelayError.Type原始值（upstream_error/channel_error/auth_error等）';
COMMENT ON COLUMN chn_error_events.error_message IS '错误详细信息';
COMMENT ON COLUMN chn_error_events.is_retryable IS '是否为可重试错误（429,500,502,503,504）';
COMMENT ON COLUMN chn_error_events.attempt IS '重试轮次编号（0=首次）';
COMMENT ON COLUMN chn_error_events.is_final IS '是否为最终错误（非中间重试失败）';
COMMENT ON COLUMN chn_error_events.latency_ms IS '请求耗时（毫秒）';
COMMENT ON COLUMN chn_error_events.created_at IS '错误发生时间';

-- 分区由 EnsurePartitions 自动管理（启动时创建当前月+未来3个月），不在迁移脚本中手动创建

-- +goose Down
DROP TABLE IF EXISTS chn_error_events;
