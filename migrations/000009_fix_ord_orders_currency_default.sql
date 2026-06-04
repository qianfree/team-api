-- +goose Up
-- 修正订单层币种：按 CLAUDE.md「三层币种固定规则」，ord_ 订单层一律 CNY（充值/套餐均为人民币）。
-- 原 DEFAULT 'USD' 与业务语义相反，且 auto_renew 历史续费订单被错误标记为 USD。

-- 1. 修正默认值：USD -> CNY
ALTER TABLE ord_orders ALTER COLUMN currency SET DEFAULT 'CNY';

-- 2. 补全字段注释，明确单位
COMMENT ON COLUMN ord_orders.currency IS '货币（订单层一律 CNY）';

-- 3. 回填历史错误数据：订单层不应存在 USD，统一修正为 CNY
UPDATE ord_orders SET currency = 'CNY' WHERE currency = 'USD';

-- +goose Down
-- 回滚：恢复原默认值与注释（历史回填的数据不再逆转）
ALTER TABLE ord_orders ALTER COLUMN currency SET DEFAULT 'USD';
COMMENT ON COLUMN ord_orders.currency IS '货币';
