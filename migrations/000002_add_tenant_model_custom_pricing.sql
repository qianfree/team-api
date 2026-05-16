-- +goose Up
-- 租户模型分配增加自定义缓存定价覆盖字段和阶梯定价支持

-- 缓存定价覆盖
ALTER TABLE mdl_tenant_models
    ADD COLUMN IF NOT EXISTS custom_cache_read_price     NUMERIC(20,10),
    ADD COLUMN IF NOT EXISTS custom_cache_creation_price  NUMERIC(20,10);

COMMENT ON COLUMN mdl_tenant_models.custom_cache_read_price IS '自定义缓存读取价格（$/1M token），NULL 表示使用基础定价';
COMMENT ON COLUMN mdl_tenant_models.custom_cache_creation_price IS '自定义缓存创建价格（$/1M token），NULL 表示使用基础定价';

-- 阶梯定价
ALTER TABLE mdl_tenant_models
    ADD COLUMN IF NOT EXISTS custom_pricing_tiers JSONB;

COMMENT ON COLUMN mdl_tenant_models.custom_pricing_tiers IS '自定义阶梯定价（JSONB 数组），格式: [{"min_tokens":0,"max_tokens":100000,"input_price":0.5,"output_price":1.5,"cache_read_price":0.1,"cache_creation_price":0.2}]';

-- +goose Down
ALTER TABLE mdl_tenant_models
    DROP COLUMN IF EXISTS custom_cache_read_price,
    DROP COLUMN IF EXISTS custom_cache_creation_price,
    DROP COLUMN IF EXISTS custom_pricing_tiers;
