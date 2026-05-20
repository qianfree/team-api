-- +goose Up
-- 更新 bil_transactions.type 注释，新增 consume（消费）类型
-- 预扣模式优化：预扣/解冻不再写流水，结算合并为一条 consume 记录
COMMENT ON COLUMN bil_transactions.type IS '类型：consume（消费）/ recharge（充值）/ adjust（调整）/ pre_deduct（预扣，已废弃）/ settle（结算，已废弃）/ refund（退款，已废弃）/ freeze（冻结，已废弃）/ unfreeze（解冻，已废弃）';

-- +goose Down
COMMENT ON COLUMN bil_transactions.type IS '类型：recharge（充值）/ pre_deduct（预扣）/ settle（结算）/ refund（退款）/ adjust（调整）/ freeze（冻结）/ unfreeze（解冻）';
