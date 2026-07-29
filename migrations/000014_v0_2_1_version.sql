-- +goose Up
-- 修正 mdl_tenant_models.multiplier 字段注释。
--
-- 原注释为「租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）」，
-- 把"模型倍率"写进了定价公式，易让人误以为该字段就是模型倍率。
-- 实际上：此字段是「租户倍率（tenant_multiplier）」来源之一，最终价格 = 基础价格 × 租户倍率。
-- （模型乘数 ModelMultiplier 当前为预留：mdl_models 无对应字段、computeCost 不纳入费用计算，恒为 1.0。）
COMMENT ON COLUMN mdl_tenant_models.multiplier IS '租户价格倍率（VIP 折扣）。作为 tenant_multiplier 参与最终价格计算：最终价格 = 基础价格 × 租户倍率。倍率来源优先级：discount_ratio > multiplier > 租户等级 price_multiplier';

-- +goose Down
COMMENT ON COLUMN mdl_tenant_models.multiplier IS '租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）';
