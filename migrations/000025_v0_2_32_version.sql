-- +goose Up
-- ============================================================
-- 模型广场重设计：推荐位 / 上新位 / 展示排序 / 价格排序基准
--
-- mdl_models：
--   * is_recommended —— 推荐位（模型广场「推荐模型」条，运营手动勾选）
--   * is_new         —— 上新位（模型广场「新上线」条，运营手动勾选，不按时间窗自动判定）
--   * display_order  —— 展示排序（推荐条内越小越靠前）
-- mdl_pricing：
--   * sort_price —— 价格排序基准价（token/tiered=输入价首档、per_request=按次单价、
--     per_second=最低档每秒单价）。pricing JSONB 是唯一真相但无法直接 ORDER BY，
--     定价写入路径同步维护本列，模型广场价格排序走索引列。
-- ============================================================

ALTER TABLE mdl_models
    ADD COLUMN IF NOT EXISTS is_recommended BOOLEAN DEFAULT FALSE NOT NULL,
    ADD COLUMN IF NOT EXISTS is_new BOOLEAN DEFAULT FALSE NOT NULL,
    ADD COLUMN IF NOT EXISTS display_order INT DEFAULT 0 NOT NULL;

COMMENT ON COLUMN mdl_models.is_recommended IS '模型广场推荐位（true=进入推荐条，运营手动勾选）';
COMMENT ON COLUMN mdl_models.is_new IS '模型广场上新位（true=进入新上线条，运营手动勾选，不按时间窗自动判定）';
COMMENT ON COLUMN mdl_models.display_order IS '展示排序（推荐条内越小越靠前，同序按 model_id）';

CREATE INDEX IF NOT EXISTS idx_mdl_models_recommended ON mdl_models (is_recommended) WHERE is_recommended;
CREATE INDEX IF NOT EXISTS idx_mdl_models_new ON mdl_models (is_new) WHERE is_new;

ALTER TABLE mdl_pricing
    ADD COLUMN IF NOT EXISTS sort_price NUMERIC(20,10);

COMMENT ON COLUMN mdl_pricing.sort_price IS '价格排序基准价（pricing JSONB 派生：token/tiered=输入价首档，per_request=按次单价，per_second=最低档每秒单价；NULL=未配价，排序落尾部）。由定价写入路径维护，存量由本迁移回填';

-- 存量回填：从 pricing JSONB 提取排序基准价（取值优先级与写入路径一致）
-- +goose StatementBegin
UPDATE mdl_pricing p
SET sort_price = COALESCE(
    NULLIF((p.pricing ->> 'input_price')::numeric, 0),
    NULLIF((p.pricing #>> '{tiers,0,input_price}')::numeric, 0),
    NULLIF((p.pricing ->> 'price')::numeric, 0),
    (
        SELECT MIN(v::numeric)
        FROM jsonb_each_text(COALESCE(p.pricing -> 'prices', '{}'::jsonb)) AS t(k, v)
        WHERE v::numeric > 0
    )
)
WHERE p.pricing IS NOT NULL
  AND p.sort_price IS NULL;
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS idx_mdl_pricing_sort_price ON mdl_pricing (sort_price);

-- +goose Down
DROP INDEX IF EXISTS idx_mdl_pricing_sort_price;

ALTER TABLE mdl_pricing
    DROP COLUMN IF EXISTS sort_price;

DROP INDEX IF EXISTS idx_mdl_models_new;
DROP INDEX IF EXISTS idx_mdl_models_recommended;

ALTER TABLE mdl_models
    DROP COLUMN IF EXISTS is_recommended,
    DROP COLUMN IF EXISTS is_new,
    DROP COLUMN IF EXISTS display_order;
