-- +goose Up
-- ============================================================
-- 修正 bil_usage_logs.tool_surcharge_cost 列注释
--
-- 工具调用附加费为固定单次计费：单价 × 次数，租户/时段/参数乘数均不
-- 作用于工具费（只作用于模型 token 费，见 billing/pricing.go
-- applyToolSurcharge 及 tool_pricing_test.go 行为锁定）。
-- 000029 首建该列时的注释误写为「已乘租户/时段乘数」，且 000029 已在
-- 存量库应用不会重跑，故以本迁移对存量库重打正确注释。纯文档性修正，
-- 不变更任何数据与结构。

COMMENT ON COLUMN bil_usage_logs.tool_surcharge_cost IS '工具调用附加费合计（按次收费，固定单次计费不乘租户/时段乘数，已含在 total_cost 中）';

-- +goose Down
-- 注释回滚无实际意义，恢复为 000029 首建时的原始文案以保持可逆
COMMENT ON COLUMN bil_usage_logs.tool_surcharge_cost IS '工具调用附加费合计（按次收费，已乘租户/时段乘数，已含在 total_cost 中）';
