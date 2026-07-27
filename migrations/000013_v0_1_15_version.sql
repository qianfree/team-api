-- +goose Up
-- 金额 decimal 化迁移（第一步）：把「非金额」小数列由 NUMERIC 改为 double precision。
--
-- 背景：系统即将把所有金额列在 Go 层映射为 decimal.Decimal（gf typeMapping: numeric→decimal 全局生效）。
-- 为避免误伤评分/利用率/延迟/告警阈值等「非金额」小数列，先把这些列显式改为 double precision，
-- 让「DB 列类型」成为「是否金额」的唯一事实源：NUMERIC=金额→decimal，double precision=非金额→float64。
-- 分类依据见 docs/float64-decimal-field-classification.md（第②类，5 表 13 列）。
--
-- 这些列当前 Go 端已是 float64，改后代码零改动；double precision 足以承载 0-100 评分、利用率、延迟、通用阈值。

-- 渠道利用率阈值（0-1 借用/抢占阈值）
ALTER TABLE chn_channels ALTER COLUMN sharing_threshold TYPE double precision USING sharing_threshold::double precision;
ALTER TABLE chn_channels ALTER COLUMN preemption_threshold TYPE double precision USING preemption_threshold::double precision;

-- 渠道健康评分（0-100 评分/延迟/稳定性）
ALTER TABLE chn_health_scores ALTER COLUMN success_rate TYPE double precision USING success_rate::double precision;
ALTER TABLE chn_health_scores ALTER COLUMN latency_ms TYPE double precision USING latency_ms::double precision;
ALTER TABLE chn_health_scores ALTER COLUMN stability_score TYPE double precision USING stability_score::double precision;
ALTER TABLE chn_health_scores ALTER COLUMN health_score TYPE double precision USING health_score::double precision;

-- 渠道健康快照（历史评分）
ALTER TABLE chn_health_snapshots ALTER COLUMN health_score TYPE double precision USING health_score::double precision;
ALTER TABLE chn_health_snapshots ALTER COLUMN success_rate TYPE double precision USING success_rate::double precision;
ALTER TABLE chn_health_snapshots ALTER COLUMN latency_ms TYPE double precision USING latency_ms::double precision;
ALTER TABLE chn_health_snapshots ALTER COLUMN stability_score TYPE double precision USING stability_score::double precision;

-- 告警阈值（通用指标阈值/触发值，对接不同量纲，非金额）
ALTER TABLE ops_alert_rules ALTER COLUMN threshold TYPE double precision USING threshold::double precision;
ALTER TABLE ops_alert_events ALTER COLUMN trigger_value TYPE double precision USING trigger_value::double precision;
ALTER TABLE ops_alert_events ALTER COLUMN threshold_value TYPE double precision USING threshold_value::double precision;

-- +goose Down
-- 回滚：恢复为原始 NUMERIC 精度（与 000001 建表一致）。

ALTER TABLE ops_alert_events ALTER COLUMN threshold_value TYPE numeric(20,10) USING threshold_value::numeric(20,10);
ALTER TABLE ops_alert_events ALTER COLUMN trigger_value TYPE numeric(20,10) USING trigger_value::numeric(20,10);
ALTER TABLE ops_alert_rules ALTER COLUMN threshold TYPE numeric(20,10) USING threshold::numeric(20,10);

ALTER TABLE chn_health_snapshots ALTER COLUMN stability_score TYPE numeric(6,2) USING stability_score::numeric(6,2);
ALTER TABLE chn_health_snapshots ALTER COLUMN latency_ms TYPE numeric(10,2) USING latency_ms::numeric(10,2);
ALTER TABLE chn_health_snapshots ALTER COLUMN success_rate TYPE numeric(6,2) USING success_rate::numeric(6,2);
ALTER TABLE chn_health_snapshots ALTER COLUMN health_score TYPE numeric(6,2) USING health_score::numeric(6,2);

ALTER TABLE chn_health_scores ALTER COLUMN health_score TYPE numeric(5,2) USING health_score::numeric(5,2);
ALTER TABLE chn_health_scores ALTER COLUMN stability_score TYPE numeric(5,2) USING stability_score::numeric(5,2);
ALTER TABLE chn_health_scores ALTER COLUMN latency_ms TYPE numeric(10,2) USING latency_ms::numeric(10,2);
ALTER TABLE chn_health_scores ALTER COLUMN success_rate TYPE numeric(5,2) USING success_rate::numeric(5,2);

ALTER TABLE chn_channels ALTER COLUMN preemption_threshold TYPE numeric(5,2) USING preemption_threshold::numeric(5,2);
ALTER TABLE chn_channels ALTER COLUMN sharing_threshold TYPE numeric(5,2) USING sharing_threshold::numeric(5,2);
