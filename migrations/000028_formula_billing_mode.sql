-- +goose Up
-- ============================================================
-- 表达式计费模式（billing_mode=formula）
--
-- mdl_pricing 新增第五种通用计费模式 formula：计费公式存 pricing JSONB
-- 顶层 formula 键（如 "ctx <= 128000 ? p*5+c*20 : p*10+c*40"），变量从
-- usage 归集（p/c 纯文本输入输出、ctx 完整上下文、cr/cc/cc1h 缓存、
-- img/ai/ao/rt 多模态细分、image_count 张数），金额 = 系数 / 1e6 ×
-- 租户乘数 × 时段乘数。保存时强制烟测（billing/formula 包）。
--
-- JSONB 天然扩展，无新列；本迁移仅更新列注释值域说明。
--
-- 部署顺序约束（同 000021 special 上线先例）：必须先部署识别 formula
-- 的代码再执行本迁移后保存 formula 定价——旧代码读到 formula 模式时
-- validatePricingConfigured 落 default 分支按未配价 fail-closed 拒绝，
-- 不会零价放行，但无法正常服务该模型。
--
-- P0 范围：仅同步请求路径（chat/responses/embeddings 等 computeCost 链路）；
-- 任务模型不支持（EstimateTaskCost 守卫拒绝）；官方参考价与租户自定义
-- 价不支持 formula 模式。

COMMENT ON COLUMN mdl_pricing.billing_mode IS '计费模式：token=按量；per_request=按次；tiered=阶梯；per_second=按秒（视频）；special=特殊计费；formula=表达式计费（pricing JSONB formula 键）';

-- +goose Down
COMMENT ON COLUMN mdl_pricing.billing_mode IS '计费模式：token=按量；per_request=按次；tiered=阶梯；per_second=按秒（视频）；special=特殊计费';
