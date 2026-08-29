-- +goose Up
-- 货币显示动态切换：将汇率配置公开（前端展示层折算与充值页折算需要读取）。
-- 已落库且 is_public=false 的行不会被 PublicSettingsGet 返回实际值（只会补注册表默认值），
-- 因此必须刷存量行；billing_currency 为新增 key，由注册表默认值兜底（USD），无需插入。

-- +goose StatementBegin
UPDATE sys_options SET is_public = true WHERE key = 'payment_exchange_rate_cny_to_usd';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE sys_options SET is_public = false WHERE key = 'payment_exchange_rate_cny_to_usd';
-- +goose StatementEnd
