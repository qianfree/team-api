-- +goose Up
-- ============================================================
-- 工具调用计费（联网搜索/云端工具按次附加费）
--
-- 渠道 adaptor 从响应证据采集工具调用计数（Claude usage.server_tool_use /
-- Gemini grounding 查询数 / OpenAI Responses web_search_call 项），
-- 结算层按全局工具价格表（sys_options: tool_call_prices，支持
-- 「工具名:模型前缀*」覆盖，内置厂商默认价见 billing/tool_pricing.go）
-- 计算按次附加费，叠加在任何计费模式之上：
--   附加费 = Σ min(次数, 10000) × 单价 × 租户乘数 × 时段乘数
-- 预扣不含工具费（调用次数事前未知），结算补差；明细随计费快照
-- billing_snapshot.tool_surcharges 留痕。
--
-- 本迁移为 bil_usage_logs 增加两列（可查询聚合，快照 JSONB 仍是完整真相）：
--   tool_usage            工具调用计数 JSONB（{"web_search": 3}）
--   tool_surcharge_cost   工具附加费合计（已含在 total_cost/actual_cost 中）

ALTER TABLE bil_usage_logs
    ADD COLUMN IF NOT EXISTS tool_usage JSONB,
    ADD COLUMN IF NOT EXISTS tool_surcharge_cost NUMERIC(20,10) DEFAULT 0 NOT NULL;

COMMENT ON COLUMN bil_usage_logs.tool_usage IS '服务端/云端工具调用计数（工具名→次数，如 {"web_search": 3}）；无工具调用为 NULL';
COMMENT ON COLUMN bil_usage_logs.tool_surcharge_cost IS '工具调用附加费合计（按次收费，已乘租户/时段乘数，已含在 total_cost 中）';

-- +goose Down
ALTER TABLE bil_usage_logs
    DROP COLUMN IF EXISTS tool_surcharge_cost,
    DROP COLUMN IF EXISTS tool_usage;
