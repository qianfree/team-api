-- +goose Up
-- 修正 mdl_tenant_models.multiplier 列注释。
--
-- 背景：本列注释历史上有两版，且线上库的实际值与迁移记录不符——
--   000001 初始版：'...最终价格 = 基础价格 × 模型倍率 × 租户倍率'
--   000014 Up 改写为带倍率优先级链的版本，但线上库至今仍是 000001 的文案
--   （2026-08-27 重跑 gf gen dao 时发现：生成结果回退掉了代码里的优先级链说明）。
--
-- 两版都已不准确，按当前实现重写：
--   1) 模型乘数是预留能力，mdl_models 无 multiplier 字段，computeCost 也不纳入，
--      bil_records.model_multiplier 快照恒为 1.0（见 pricing.go 的「4.5 模型倍率」段）；
--   2) 000016 引入时段定价后，computeCost 实际乘的是 TenantMultiplier × TimeMultiplier
--      （pricing.go:381 / 406 / 546），旧文案均未体现时段乘数。

COMMENT ON COLUMN mdl_tenant_models.multiplier IS '租户价格倍率（VIP 折扣）。作为 tenant_multiplier 参与计费，当前实际生效公式 = 基础价格 × 租户乘数 × 时段乘数；模型乘数为预留能力（mdl_models 无 multiplier 字段、computeCost 未接入），bil_records.model_multiplier 快照恒为 1.0。倍率来源优先级：discount_ratio > multiplier > 租户等级 price_multiplier（级别兜底仅在前两者均未设置、且级别倍率 <1.0 时生效）';

-- 渠道健康度：修正与实现不符的列注释，并把两个僵尸列标记为已废弃。
--
-- 背景：健康体系已重写为「Redis 渠道×模型 EWMA 为唯一真相 + 每 5 分钟聚合落盘」，
-- 但 chn_health_scores 的列注释仍停留在旧的加权公式，读注释会得到完全错误的结论。
--   实际公式：health_score = avg(succ_ewma)^α × 100
--            （α = 路由策略 health.alpha，默认 2；均值只统计有真实上报的模型）
--   注释声称：成功率×0.40 + 延迟分×0.25 + 稳定性×0.20 + 连续失败分×0.15
--
-- stability_score / consecutive_failures 自重写后从未被更新过，只在建渠道
-- （InitHealthScore）和重置健康度时写入常量，趋势图与调度均不消费。保留列不删除
-- （避免破坏已有数据与历史快照），改为注释标注废弃 + 代码停止写入。

COMMENT ON COLUMN chn_health_scores.success_rate IS '成功率（0-100）= 该渠道各模型 succ_ewma 均值×100，仅统计有真实上报的模型';
COMMENT ON COLUMN chn_health_scores.latency_ms IS '平均延迟（毫秒）= 该渠道各模型 lat_ewma 均值，仅统计有真实上报的模型；仅展示用，不参与健康分计算';
COMMENT ON COLUMN chn_health_scores.health_score IS '综合健康度（0-100）= avg(succ_ewma)^α × 100，α 取路由策略 health.alpha（默认 2），与调度 healthFactor 同源；每 5 分钟由维护任务聚合落盘，另可由渠道测试/重置健康度即时触发；Redis 读失败或全部模型无真实上报时保留旧值不覆盖。调度决策不读此表，仅供管理后台展示';
COMMENT ON COLUMN chn_health_scores.calculated_at IS '最近一次聚合落盘时间';
COMMENT ON COLUMN chn_health_scores.stability_score IS '【已废弃】稳定性评分，健康体系重写后不再写入，保留列仅为兼容历史数据';
COMMENT ON COLUMN chn_health_scores.consecutive_failures IS '【已废弃】连续失败次数，已由 Redis 熔断器的滑动窗口计数取代（dispatch:v1:breaker:*），保留列仅为兼容历史数据';

COMMENT ON COLUMN chn_health_snapshots.health_score IS '综合健康度（0-100）快照，取自 chn_health_scores.health_score';
COMMENT ON COLUMN chn_health_snapshots.stability_score IS '【已废弃】稳定性评分，不再写入，新快照为 0';
COMMENT ON COLUMN chn_health_snapshots.consecutive_failures IS '【已废弃】连续失败次数，不再写入，新快照为 0';

-- stability_score 为 NOT NULL 且无默认值，代码停止写入后插入会失败，补默认值。
-- consecutive_failures 已有 DEFAULT 0，无需处理。
ALTER TABLE chn_health_snapshots
    ALTER COLUMN stability_score SET DEFAULT 0;


-- +goose Down
ALTER TABLE chn_health_snapshots
    ALTER COLUMN stability_score DROP DEFAULT;

COMMENT ON COLUMN chn_health_snapshots.consecutive_failures IS '连续失败次数';
COMMENT ON COLUMN chn_health_snapshots.stability_score IS '稳定性评分（0-100）';
COMMENT ON COLUMN chn_health_snapshots.health_score IS '综合健康度（0-100）';

COMMENT ON COLUMN chn_health_scores.consecutive_failures IS '连续失败次数（成功后归零）';
COMMENT ON COLUMN chn_health_scores.stability_score IS '稳定性评分（0-100，基于延迟波动计算）';
COMMENT ON COLUMN chn_health_scores.calculated_at IS '最近一次计算时间';
COMMENT ON COLUMN chn_health_scores.health_score IS '综合健康度（0-100）= 成功率×0.40 + 延迟分×0.25 + 稳定性×0.20 + 连续失败分×0.15';
COMMENT ON COLUMN chn_health_scores.latency_ms IS '平均延迟（毫秒）';
COMMENT ON COLUMN chn_health_scores.success_rate IS '请求成功率（0-100）';

COMMENT ON COLUMN mdl_tenant_models.multiplier IS '租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）';
