// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnHealthScores is the golang structure for table chn_health_scores.
type ChnHealthScores struct {
	Id                  int64       `json:"id"                   orm:"id"                   description:"主键ID"`                                                                                                                                                                // 主键ID
	ChannelId           int64       `json:"channel_id"           orm:"channel_id"           description:"关联渠道ID"`                                                                                                                                                              // 关联渠道ID
	SuccessRate         float64     `json:"success_rate"         orm:"success_rate"         description:"成功率（0-100）= 该渠道各模型 succ_ewma 均值×100，仅统计有真实上报的模型"`                                                                                                                     // 成功率（0-100）= 该渠道各模型 succ_ewma 均值×100，仅统计有真实上报的模型
	LatencyMs           float64     `json:"latency_ms"           orm:"latency_ms"           description:"平均延迟（毫秒）= 该渠道各模型 lat_ewma 均值，仅统计有真实上报的模型；仅展示用，不参与健康分计算"`                                                                                                              // 平均延迟（毫秒）= 该渠道各模型 lat_ewma 均值，仅统计有真实上报的模型；仅展示用，不参与健康分计算
	StabilityScore      float64     `json:"stability_score"      orm:"stability_score"      description:"【已废弃】稳定性评分，健康体系重写后不再写入，保留列仅为兼容历史数据"`                                                                                                                                  // 【已废弃】稳定性评分，健康体系重写后不再写入，保留列仅为兼容历史数据
	ConsecutiveFailures int         `json:"consecutive_failures" orm:"consecutive_failures" description:"【已废弃】连续失败次数，已由 Redis 熔断器的滑动窗口计数取代（dispatch:v1:breaker:*），保留列仅为兼容历史数据"`                                                                                                // 【已废弃】连续失败次数，已由 Redis 熔断器的滑动窗口计数取代（dispatch:v1:breaker:*），保留列仅为兼容历史数据
	HealthScore         float64     `json:"health_score"         orm:"health_score"         description:"综合健康度（0-100）= avg(succ_ewma)^α × 100，α 取路由策略 health.alpha（默认 2），与调度 healthFactor 同源；每 5 分钟由维护任务聚合落盘，另可由渠道测试/重置健康度即时触发；Redis 读失败或全部模型无真实上报时保留旧值不覆盖。调度决策不读此表，仅供管理后台展示"` // 综合健康度（0-100）= avg(succ_ewma)^α × 100，α 取路由策略 health.alpha（默认 2），与调度 healthFactor 同源；每 5 分钟由维护任务聚合落盘，另可由渠道测试/重置健康度即时触发；Redis 读失败或全部模型无真实上报时保留旧值不覆盖。调度决策不读此表，仅供管理后台展示
	CalculatedAt        *gtime.Time `json:"calculated_at"        orm:"calculated_at"        description:"最近一次聚合落盘时间"`                                                                                                                                                          // 最近一次聚合落盘时间
	CreatedAt           *gtime.Time `json:"created_at"           orm:"created_at"           description:"创建时间"`                                                                                                                                                                // 创建时间
	UpdatedAt           *gtime.Time `json:"updated_at"           orm:"updated_at"           description:"更新时间"`                                                                                                                                                                // 更新时间
}
