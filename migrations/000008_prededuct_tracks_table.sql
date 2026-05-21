-- +goose Up
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

-- 部分索引：只索引活跃记录，避免历史数据膨胀
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
DROP TABLE IF EXISTS bil_prededuct_tracks;
