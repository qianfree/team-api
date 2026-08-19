-- +goose Up
-- 模型时段定价：mdl_pricing 增加时段定价 JSONB 列（峰谷定价/工作时段/限时促销）
ALTER TABLE mdl_pricing
    ADD COLUMN IF NOT EXISTS time_segments JSONB;

COMMENT ON COLUMN mdl_pricing.time_segments IS '时段定价（JSONB 有序数组，仅 min_tokens=0 锚点行生效）：[{"name":"闲时","days":[1,2,3,4,5],"start_time":"00:00","end_time":"08:00","valid_from":"","valid_to":"","multiplier":0.5}]，按序先命中先生效，未命中=默认价（乘数 1.0），days 1=周一..7=周日 空=每天，end<start 表示跨零点';

-- +goose Down
ALTER TABLE mdl_pricing
    DROP COLUMN IF EXISTS time_segments;
