-- +goose Up
-- Webhook 功能完善 + 余额预警事件驱动改造
-- 合并原 000007 ~ 000010 四个迁移文件

-- ========================================
-- 1. Webhook 性能优化：补充索引
-- ========================================

-- JSONB GIN 索引：支持 webhook 配置的 events 查询
CREATE INDEX IF NOT EXISTS idx_opn_webhook_configs_events_gin ON opn_webhook_configs USING gin (events jsonb_path_ops);

-- 复合索引：加速查找租户的活跃 webhook 配置
CREATE INDEX IF NOT EXISTS idx_opn_webhook_configs_tenant_active ON opn_webhook_configs (tenant_id, is_active) WHERE is_active = true;

-- 按配置+租户查询投递日志
CREATE INDEX IF NOT EXISTS idx_opn_webhook_delivery_logs_config ON opn_webhook_delivery_logs (webhook_config_id, tenant_id, created_at DESC);

-- 按配置查询事件（级联删除时使用）
CREATE INDEX IF NOT EXISTS idx_opn_webhook_events_config ON opn_webhook_events (webhook_config_id);

-- BRIN 索引：追加写入表磁盘友好
CREATE INDEX IF NOT EXISTS idx_opn_webhook_events_created_brin ON opn_webhook_events USING brin (created_at);
CREATE INDEX IF NOT EXISTS idx_opn_webhook_delivery_logs_created_brin ON opn_webhook_delivery_logs USING brin (created_at);

-- ========================================
-- 2. 放宽 webhook URL 约束：允许 http:// 和 https://
-- ========================================
ALTER TABLE opn_webhook_configs DROP CONSTRAINT IF EXISTS chk_opn_webhook_configs_url;

-- ========================================
-- 3. 钱包表增加低余额预警推送标记
-- ========================================
ALTER TABLE bil_wallets ADD COLUMN IF NOT EXISTS low_balance_notified BOOLEAN NOT NULL DEFAULT false;
COMMENT ON COLUMN bil_wallets.low_balance_notified IS '低余额预警是否已推送（充值恢复后重置为 false）';

-- ========================================
-- 4. 修正通知模板占位符语法
-- ========================================
UPDATE ntf_templates
SET body_template = '<p>您的账户可用余额为 <strong>{{.available}}</strong> USD，已低于预警线 <strong>{{.threshold}}</strong> USD。</p><p>请及时充值以避免服务中断。</p>'
WHERE code = 'balance_warning';

-- +goose Down
-- 恢复通知模板占位符
UPDATE ntf_templates
SET body_template = '<p>您的账户可用余额为 <strong>${available}</strong> USD，已低于预警线 <strong>${threshold}</strong> USD。</p><p>请及时充值以避免服务中断。</p>'
WHERE code = 'balance_warning';

-- 移除钱包低余额标记字段
ALTER TABLE bil_wallets DROP COLUMN IF EXISTS low_balance_notified;

-- 恢复仅允许 https://
ALTER TABLE opn_webhook_configs DROP CONSTRAINT IF EXISTS chk_opn_webhook_configs_url;

-- 移除索引
DROP INDEX IF EXISTS idx_opn_webhook_delivery_logs_created_brin;
DROP INDEX IF EXISTS idx_opn_webhook_events_created_brin;
DROP INDEX IF EXISTS idx_opn_webhook_events_config;
DROP INDEX IF EXISTS idx_opn_webhook_delivery_logs_config;
DROP INDEX IF EXISTS idx_opn_webhook_configs_tenant_active;
DROP INDEX IF EXISTS idx_opn_webhook_configs_events_gin;
