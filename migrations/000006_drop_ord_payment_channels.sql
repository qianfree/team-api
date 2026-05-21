-- +goose Up
DROP TABLE IF EXISTS ord_payment_channels;

-- +goose Down
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
