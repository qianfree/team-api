-- +goose Up
-- 模型时段定价：mdl_pricing 增加时段定价 JSONB 列（峰谷定价/工作时段/限时促销）
ALTER TABLE mdl_pricing
    ADD COLUMN IF NOT EXISTS time_segments JSONB;

COMMENT ON COLUMN mdl_pricing.time_segments IS '时段定价（JSONB 有序数组，仅 min_tokens=0 锚点行生效）：[{"name":"闲时","days":[1,2,3,4,5],"start_time":"00:00","end_time":"08:00","valid_from":"","valid_to":"","multiplier":0.5}]，按序先命中先生效，未命中=默认价（乘数 1.0），days 1=周一..7=周日 空=每天，end<start 表示跨零点';

-- 模型定价展示字段：价格说明（内部）+ 折扣标签 / 价格调整说明（对外展示）
ALTER TABLE mdl_pricing
    ADD COLUMN IF NOT EXISTS price_note VARCHAR(500),
    ADD COLUMN IF NOT EXISTS discount_label VARCHAR(50),
    ADD COLUMN IF NOT EXISTS price_change_note VARCHAR(200);

COMMENT ON COLUMN mdl_pricing.price_note IS '价格说明（仅管理后台可见，调价背景等内部备注），仅 min_tokens=0 锚点行使用，NULL=无';
COMMENT ON COLUMN mdl_pricing.discount_label IS '折扣标签（对外展示，如"7折起"、"限时5折"），仅 min_tokens=0 锚点行使用，NULL/空=不展示';
COMMENT ON COLUMN mdl_pricing.price_change_note IS '价格调整说明（对外展示，提示用户价格有变动，如"9月1日起输入价下调"），仅 min_tokens=0 锚点行使用，NULL/空=不展示';

-- 渠道调试日志：渠道调试开关开启时，记录经该渠道每次请求尝试（per-attempt，failover 各自成一条）
-- 的四段完整报文（客户端↔系统↔上游）。调试用途：body 不截断、无自动过期，由管理员手动删除/清空。
-- 不按月分区（需按渠道硬删，分区反而碍事）；BRIN 应对按时间扫描。

CREATE TABLE IF NOT EXISTS chn_debug_logs (
    id                     BIGSERIAL PRIMARY KEY,
    channel_id             BIGINT       NOT NULL,
    channel_name           VARCHAR(128),
    channel_type           INT,
    request_id             VARCHAR(64)  NOT NULL DEFAULT '',
    tenant_id              BIGINT,
    user_id                BIGINT,
    api_key_id             BIGINT,
    model_name             VARCHAR(128),
    upstream_model         VARCHAR(128),
    relay_mode             VARCHAR(32),
    inbound_path           VARCHAR(128),
    upstream_url           TEXT,
    is_stream              BOOLEAN      NOT NULL DEFAULT FALSE,
    retry_index            INT          NOT NULL DEFAULT 0,
    is_final               BOOLEAN      NOT NULL DEFAULT TRUE,
    upstream_status_code   INT,
    client_status_code     INT,
    error                  TEXT,
    client_req_headers     JSONB,
    client_req_body        TEXT,
    client_req_encoding    VARCHAR(8)   NOT NULL DEFAULT 'plain',
    upstream_req_headers   JSONB,
    upstream_req_body      TEXT,
    upstream_req_encoding  VARCHAR(8)   NOT NULL DEFAULT 'plain',
    upstream_resp_headers  JSONB,
    upstream_resp_body     TEXT,
    upstream_resp_encoding VARCHAR(8)   NOT NULL DEFAULT 'plain',
    client_resp_headers    JSONB,
    client_resp_body       TEXT,
    client_resp_encoding   VARCHAR(8)   NOT NULL DEFAULT 'plain',
    upstream_latency_ms    INT,
    total_latency_ms       INT,
    first_token_ms         INT,
    client_req_bytes       BIGINT,
    upstream_req_bytes     BIGINT,
    upstream_resp_bytes    BIGINT,
    client_resp_bytes      BIGINT,
    conversion             JSONB,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chn_debug_logs_channel_id   ON chn_debug_logs (channel_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_chn_debug_logs_request_id   ON chn_debug_logs (request_id);
CREATE INDEX IF NOT EXISTS idx_chn_debug_logs_created_brin ON chn_debug_logs USING BRIN (created_at);

COMMENT ON TABLE chn_debug_logs IS '渠道调试日志：调试开关开启的渠道，per-attempt 记录客户端↔系统↔上游四段完整报文（头/体/编码），敏感凭证脱敏，二进制体 base64；body 不截断、无自动过期，按渠道手动清理';
COMMENT ON COLUMN chn_debug_logs.id IS '主键ID';
COMMENT ON COLUMN chn_debug_logs.channel_id IS '渠道ID（逻辑关联 chn_channels.id，无外键）';
COMMENT ON COLUMN chn_debug_logs.channel_name IS '渠道名称（冗余存储，渠道删除后仍可辨识）';
COMMENT ON COLUMN chn_debug_logs.channel_type IS '渠道类型（ProviderType 枚举值）';
COMMENT ON COLUMN chn_debug_logs.request_id IS '请求唯一ID（同一请求多次重试共享）';
COMMENT ON COLUMN chn_debug_logs.tenant_id IS '租户ID';
COMMENT ON COLUMN chn_debug_logs.user_id IS '用户ID';
COMMENT ON COLUMN chn_debug_logs.api_key_id IS 'API Key ID';
COMMENT ON COLUMN chn_debug_logs.model_name IS '用户请求的模型名';
COMMENT ON COLUMN chn_debug_logs.upstream_model IS '上游实际使用的模型名（模型映射后）';
COMMENT ON COLUMN chn_debug_logs.relay_mode IS '转发模式（chat_completions/claude_messages/embeddings 等）';
COMMENT ON COLUMN chn_debug_logs.inbound_path IS '入站端点路径（如 /v1/chat/completions）';
COMMENT ON COLUMN chn_debug_logs.upstream_url IS '上游请求 URL（query 中的凭证参数已脱敏）';
COMMENT ON COLUMN chn_debug_logs.is_stream IS '是否流式请求';
COMMENT ON COLUMN chn_debug_logs.retry_index IS '重试轮次（0=首次尝试）';
COMMENT ON COLUMN chn_debug_logs.is_final IS '是否为产生客户端响应的最终尝试（成功/终止/流中断）';
COMMENT ON COLUMN chn_debug_logs.upstream_status_code IS '上游 HTTP 状态码（未发起请求或连接失败为 NULL）';
COMMENT ON COLUMN chn_debug_logs.client_status_code IS '返回客户端的状态码';
COMMENT ON COLUMN chn_debug_logs.error IS '本尝试的错误信息（成功为空）';
COMMENT ON COLUMN chn_debug_logs.client_req_headers IS '段1 客户端请求头（凭证类头脱敏：前6后4）JSON';
COMMENT ON COLUMN chn_debug_logs.client_req_body IS '段1 客户端请求体（完整不截断；二进制为 base64，见 encoding 列）';
COMMENT ON COLUMN chn_debug_logs.client_req_encoding IS '段1 编码：plain / base64';
COMMENT ON COLUMN chn_debug_logs.upstream_req_headers IS '段2 发往上游的最终请求头（协议转换+override 后，凭证类脱敏）JSON';
COMMENT ON COLUMN chn_debug_logs.upstream_req_body IS '段2 发往上游的请求体（实际发送字节，含协议转换；二进制为 base64）';
COMMENT ON COLUMN chn_debug_logs.upstream_req_encoding IS '段2 编码：plain / base64';
COMMENT ON COLUMN chn_debug_logs.upstream_resp_headers IS '段3 上游响应头 JSON（Content-Encoding 可能已被 net/http 透明解压移除）';
COMMENT ON COLUMN chn_debug_logs.upstream_resp_body IS '段3 上游响应体（Go 透明解压后的字节；流式为 SSE 原文；二进制为 base64）';
COMMENT ON COLUMN chn_debug_logs.upstream_resp_encoding IS '段3 编码：plain / base64';
COMMENT ON COLUMN chn_debug_logs.client_resp_headers IS '段4 返回客户端的响应头 JSON';
COMMENT ON COLUMN chn_debug_logs.client_resp_body IS '段4 返回客户端的响应体（协议转换后，完整不截断；二进制为 base64）';
COMMENT ON COLUMN chn_debug_logs.client_resp_encoding IS '段4 编码：plain / base64';
COMMENT ON COLUMN chn_debug_logs.upstream_latency_ms IS '上游往返耗时（RoundTrip 到响应头）毫秒';
COMMENT ON COLUMN chn_debug_logs.total_latency_ms IS '请求总耗时毫秒';
COMMENT ON COLUMN chn_debug_logs.first_token_ms IS '首字节延迟毫秒';
COMMENT ON COLUMN chn_debug_logs.client_req_bytes IS '段1 请求体原始字节数（base64 落库膨胀前的真实大小）';
COMMENT ON COLUMN chn_debug_logs.upstream_req_bytes IS '段2 请求体原始字节数';
COMMENT ON COLUMN chn_debug_logs.upstream_resp_bytes IS '段3 响应体原始字节数';
COMMENT ON COLUMN chn_debug_logs.client_resp_bytes IS '段4 响应体原始字节数';
COMMENT ON COLUMN chn_debug_logs.conversion IS '协议转换信息 JSON：client_format 客户端协议 / upstream_format 上游协议 / chain 请求侧转换链 / bridge 桥接方式（responses_api|responses_direct|pass_through，空=常规转换）';
COMMENT ON COLUMN chn_debug_logs.created_at IS '创建时间';


-- +goose Down
DROP TABLE IF EXISTS chn_debug_logs;

ALTER TABLE mdl_pricing
    DROP COLUMN IF EXISTS time_segments;

ALTER TABLE mdl_pricing
DROP COLUMN IF EXISTS price_change_note,
    DROP COLUMN IF EXISTS discount_label,
    DROP COLUMN IF EXISTS price_note;