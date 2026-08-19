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


-- +goose Down
ALTER TABLE mdl_pricing
    DROP COLUMN IF EXISTS time_segments;

ALTER TABLE mdl_pricing
DROP COLUMN IF EXISTS price_change_note,
    DROP COLUMN IF EXISTS discount_label,
    DROP COLUMN IF EXISTS price_note;