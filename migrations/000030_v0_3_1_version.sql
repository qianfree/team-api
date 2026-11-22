-- +goose Up
-- ============================================================================
-- 租户定价统一为 custom_pricing 单一 PricingBlob 形状 JSONB（单版本原子收口）
--
-- 设计文档：docs/租户定价覆盖blob统一设计.md
--
-- 变更内容（按序执行，一次完成）：
--   1. 回填：扁平价格列（含模式覆盖与阶梯 JSONB）→ custom_pricing blob 键。
--      旧 TenantPricingPatch 三键（prices/time_segments/param_multipliers）
--      与 PricingBlob 键名同形，无需迁移原样保留。
--   2. 一致性断言：列值与 blob 键逐项比对，不一致即 RAISE 中止
--      （防「迁移与部署拆开执行、旧代码窗口期写列」的数据丢失）。
--   3. 删除全部扁平列：custom_pricing blob 成为租户定价覆盖的唯一存储。
--
-- 覆盖语义（读侧 GetModelPriceAt / 写侧 ValidateTenantPricingBlob 同口径）：
--   键存在正值 = 覆盖；键缺失 = 继承平台；time_segments/param_multipliers
--   支持空数组 = 显式关闭；价格键与 discount_ratio 互斥（专属价一口价不打折）。
--
-- ⚠️ 发布要求：goose 随新二进制启动原子执行（单实例停机发布）。若迁移被
-- 提前单独执行，旧代码继续写列的值会被步骤 2 断言拦下——处置：注释掉
-- 步骤 2/3 重跑本文件（步骤 1 幂等）补齐回填后恢复完整文件再执行。
-- ============================================================================

-- 1. 回填（幂等：jsonb || 同值重写无副作用；jsonb_strip_nulls 剔除 NULL 键 =
--    未覆盖不产生键；billing_mode 空串视同未覆盖）
UPDATE mdl_tenant_models
SET custom_pricing = COALESCE(custom_pricing, '{}'::jsonb)
    || jsonb_strip_nulls(jsonb_build_object(
        'input_price',              custom_input_price,
        'output_price',             custom_output_price,
        'cache_read_price',         custom_cache_read_price,
        'cache_creation_price',     custom_cache_creation_price,
        'price',                    per_request_price,
        'billing_mode',             nullif(billing_mode, '')
    ))
    -- 阶梯整体替换（空数组/null 无覆盖语义，不并入）
    || CASE WHEN custom_pricing_tiers IS NOT NULL
             AND custom_pricing_tiers NOT IN ('null', '[]')
            THEN jsonb_build_object('tiers', custom_pricing_tiers::jsonb)
            ELSE '{}'::jsonb END
WHERE custom_input_price          IS NOT NULL
   OR custom_output_price         IS NOT NULL
   OR custom_cache_read_price     IS NOT NULL
   OR custom_cache_creation_price IS NOT NULL
   OR per_request_price           IS NOT NULL
   OR nullif(billing_mode, '')    IS NOT NULL
   OR (custom_pricing_tiers IS NOT NULL AND custom_pricing_tiers NOT IN ('null', '[]'));

-- 2. 一致性断言：任一列有值而 blob 键缺失/不等 → 中止
-- +goose StatementBegin
DO $$
DECLARE
    mismatch_count integer;
BEGIN
    SELECT count(*) INTO mismatch_count
    FROM mdl_tenant_models
    WHERE (custom_input_price IS NOT NULL AND (custom_pricing ->> 'input_price') IS DISTINCT FROM custom_input_price::text)
       OR (custom_output_price IS NOT NULL AND (custom_pricing ->> 'output_price') IS DISTINCT FROM custom_output_price::text)
       OR (custom_cache_read_price IS NOT NULL AND (custom_pricing ->> 'cache_read_price') IS DISTINCT FROM custom_cache_read_price::text)
       OR (custom_cache_creation_price IS NOT NULL AND (custom_pricing ->> 'cache_creation_price') IS DISTINCT FROM custom_cache_creation_price::text)
       OR (per_request_price IS NOT NULL AND (custom_pricing ->> 'price') IS DISTINCT FROM per_request_price::text)
       OR (nullif(billing_mode, '') IS NOT NULL AND (custom_pricing ->> 'billing_mode') IS DISTINCT FROM billing_mode)
       OR (custom_pricing_tiers NOT IN ('null', '[]') AND custom_pricing_tiers IS NOT NULL
           AND (custom_pricing -> 'tiers') IS DISTINCT FROM custom_pricing_tiers::jsonb);
    IF mismatch_count > 0 THEN
        RAISE EXCEPTION 'mdl_tenant_models 扁平列与 custom_pricing blob 不一致：% 行（步骤 1 回填应已覆盖，检查窗口期写入后重跑回填 UPDATE）', mismatch_count;
    END IF;
END $$;
-- +goose StatementEnd

-- 3. 删除扁平列（价格 + 模式覆盖 + 阶梯；折扣/并发/渠道/展示字段保留）
--    DROP IF EXISTS：custom_cache_creation_1h_price 仅存在于个别开发库，线上无此列
ALTER TABLE mdl_tenant_models
    DROP COLUMN IF EXISTS custom_input_price,
    DROP COLUMN IF EXISTS custom_output_price,
    DROP COLUMN IF EXISTS custom_cache_read_price,
    DROP COLUMN IF EXISTS custom_cache_creation_price,
    DROP COLUMN IF EXISTS custom_cache_creation_1h_price,
    DROP COLUMN IF EXISTS per_request_price,
    DROP COLUMN IF EXISTS billing_mode,
    DROP COLUMN IF EXISTS custom_pricing_tiers;

COMMENT ON COLUMN mdl_tenant_models.custom_pricing IS '定价覆盖 JSONB（PricingBlob 形状，唯一入口）：键存在=覆盖/缺失=继承；含旧补丁三键（prices/time_segments/param_multipliers）同形兼容；价格键与 discount_ratio 互斥（专属价一口价，写侧校验拒绝并存）';

-- +goose Down
-- ============================================================================
-- 回滚：从 blob 反向重建扁平列（含回滚前新代码写入的 blob-only 覆盖），
-- 供旧版本二进制读取；随后剥离 blob 中的回填价格键，恢复迁移前状态
--（patch 三键保留——旧代码写侧本来就走它们）。
-- 回滚流程：goose down → 回退旧二进制。
-- ============================================================================
ALTER TABLE mdl_tenant_models
    ADD COLUMN IF NOT EXISTS custom_input_price          NUMERIC(20,10),
    ADD COLUMN IF NOT EXISTS custom_output_price         NUMERIC(20,10),
    ADD COLUMN IF NOT EXISTS custom_cache_read_price     NUMERIC(20,10),
    ADD COLUMN IF NOT EXISTS custom_cache_creation_price NUMERIC(20,10),
    ADD COLUMN IF NOT EXISTS per_request_price           NUMERIC(20,10),
    ADD COLUMN IF NOT EXISTS billing_mode                VARCHAR(20),
    ADD COLUMN IF NOT EXISTS custom_pricing_tiers        JSONB;

-- 先重建列值（含 blob-only 新写入），再剥离 blob 价格键——两步顺序不可颠倒
UPDATE mdl_tenant_models SET
    custom_input_price          = NULLIF(custom_pricing ->> 'input_price', '')::numeric,
    custom_output_price         = NULLIF(custom_pricing ->> 'output_price', '')::numeric,
    custom_cache_read_price     = NULLIF(custom_pricing ->> 'cache_read_price', '')::numeric,
    custom_cache_creation_price = NULLIF(custom_pricing ->> 'cache_creation_price', '')::numeric,
    per_request_price           = NULLIF(custom_pricing ->> 'price', '')::numeric,
    billing_mode                = NULLIF(custom_pricing ->> 'billing_mode', ''),
    custom_pricing_tiers        = COALESCE(custom_pricing -> 'tiers', 'null'::jsonb),
    custom_pricing              = custom_pricing
        - 'input_price' - 'output_price' - 'cache_read_price' - 'cache_creation_price'
        - 'cache_creation_1h_price' - 'price' - 'billing_mode' - 'tiers'
WHERE custom_pricing IS NOT NULL;

COMMENT ON COLUMN mdl_tenant_models.custom_input_price IS '自定义输入价格（$/1M token），NULL 表示使用基础定价 [down 重建，仅供旧版本回滚]';
