-- +goose Up
-- 渠道调度重构（开发计划 §4.1，基线方案 §11）：
-- 1. tier：三档固定层级，替代自由整数优先级的调度语义（priority 列保留不删，仅停止参与调度，供回滚与历史查询）
-- 2. strict_capacity：修订 R4，Redis 故障时该渠道 fail-closed（实例级保守限额），默认 fail-open
-- 3. cost_ratio：渠道×模型成本比例（无量纲），参与调度 costFactor 计算

ALTER TABLE chn_channels ADD COLUMN IF NOT EXISTS tier VARCHAR(16) NOT NULL DEFAULT 'primary';
COMMENT ON COLUMN chn_channels.tier IS '调度层级：primary=首选 secondary=备用 reserve=保底';

ALTER TABLE chn_channels ADD COLUMN IF NOT EXISTS strict_capacity BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN chn_channels.strict_capacity IS '严格容量：true 时 Redis 故障期间使用实例级保守并发限额（fail-closed），用于高成本/严格配额渠道；false 为 fail-open';

ALTER TABLE chn_abilities ADD COLUMN IF NOT EXISTS cost_ratio NUMERIC(10,4) NOT NULL DEFAULT 1.0;
COMMENT ON COLUMN chn_abilities.cost_ratio IS '成本比例：该渠道该模型上游实际价/平台基准价，1.0=等价，0.8=八折，参与调度 costFactor 计算（无量纲比例，非金额）';

-- 按现有 priority 自动映射三档：全局最高优先级值 → primary，次高 → secondary，其余 → reserve
-- +goose StatementBegin
WITH ranked AS (
	SELECT id, DENSE_RANK() OVER (ORDER BY priority DESC) AS rk FROM chn_channels
)
UPDATE chn_channels c SET tier = CASE ranked.rk
	WHEN 1 THEN 'primary'
	WHEN 2 THEN 'secondary'
	ELSE 'reserve' END
FROM ranked WHERE c.id = ranked.id;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE chn_abilities DROP COLUMN IF EXISTS cost_ratio;
ALTER TABLE chn_channels DROP COLUMN IF EXISTS strict_capacity;
ALTER TABLE chn_channels DROP COLUMN IF EXISTS tier;
