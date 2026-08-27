-- +goose Up
-- 修正 mdl_tenant_models.multiplier 列注释。
--
-- 背景：本列注释历史上有两版，且线上库的实际值与迁移记录不符——
--   000001 初始版：'...最终价格 = 基础价格 × 模型倍率 × 租户倍率'
--   000014 Up 改写为带倍率优先级链的版本，但线上库至今仍是 000001 的文案
--   （2026-08-27 重跑 gf gen dao 时发现：生成结果回退掉了代码里的优先级链说明）。
--
-- 两版都已不准确，按当前实现重写：
--   1) 模型乘数是预留能力，mdl_models 无 multiplier 字段，computeCost 也不纳入，
--      bil_records.model_multiplier 快照恒为 1.0（见 pricing.go 的「4.5 模型倍率」段）；
--   2) 000016 引入时段定价后，computeCost 实际乘的是 TenantMultiplier × TimeMultiplier
--      （pricing.go:381 / 406 / 546），旧文案均未体现时段乘数。

COMMENT ON COLUMN mdl_tenant_models.multiplier IS '租户价格倍率（VIP 折扣）。作为 tenant_multiplier 参与计费，当前实际生效公式 = 基础价格 × 租户乘数 × 时段乘数；模型乘数为预留能力（mdl_models 无 multiplier 字段、computeCost 未接入），bil_records.model_multiplier 快照恒为 1.0。倍率来源优先级：discount_ratio > multiplier > 租户等级 price_multiplier（级别兜底仅在前两者均未设置、且级别倍率 <1.0 时生效）';

-- +goose Down
COMMENT ON COLUMN mdl_tenant_models.multiplier IS '租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）';
