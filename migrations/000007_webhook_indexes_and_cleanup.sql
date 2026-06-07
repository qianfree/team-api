-- +goose Up
-- Webhook 性能优化：补充索引 + 事件清理策略
--   1. opn_webhook_configs: GIN 索引支持 JSONB @> 查询，复合索引加速活跃配置查找
--   2. opn_webhook_delivery_logs: 按配置+租户查询索引
--   3. opn_webhook_events: 按配置查询索引（级联删除用），BRIN 索引支持追加写入

-- 1. JSONB GIN 索引：支持 PublishWebhookEvent 中的 events::jsonb @> ? 查询
CREATE INDEX idx_opn_webhook_configs_events_gin ON opn_webhook_configs USING gin (events jsonb_path_ops);

-- 2. 复合索引：加速查找租户的活跃 webhook 配置
CREATE INDEX idx_opn_webhook_configs_tenant_active ON opn_webhook_configs (tenant_id, is_active) WHERE is_active = true;

-- 3. 按配置+租户查询投递日志（WebhookDeliveryLogs API 使用）
CREATE INDEX idx_opn_webhook_delivery_logs_config ON opn_webhook_delivery_logs (webhook_config_id, tenant_id, created_at DESC);

-- 4. 按配置查询事件（级联删除时使用）
CREATE INDEX idx_opn_webhook_events_config ON opn_webhook_events (webhook_config_id);

-- 5. BRIN 索引：events 表追加写入模式，磁盘友好
CREATE INDEX idx_opn_webhook_events_created_brin ON opn_webhook_events USING brin (created_at);

-- 6. BRIN 索引：delivery_logs 表追加写入模式
CREATE INDEX idx_opn_webhook_delivery_logs_created_brin ON opn_webhook_delivery_logs USING brin (created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_opn_webhook_delivery_logs_created_brin;
DROP INDEX IF EXISTS idx_opn_webhook_events_created_brin;
DROP INDEX IF EXISTS idx_opn_webhook_events_config;
DROP INDEX IF EXISTS idx_opn_webhook_delivery_logs_config;
DROP INDEX IF EXISTS idx_opn_webhook_configs_tenant_active;
DROP INDEX IF EXISTS idx_opn_webhook_configs_events_gin;
