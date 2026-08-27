// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnHealthScores is the golang structure of table chn_health_scores for DAO operations like Where/Data.
type ChnHealthScores struct {
	g.Meta              `orm:"table:chn_health_scores, do:true"`
	Id                  any         // 主键ID
	ChannelId           any         // 关联渠道ID
	SuccessRate         any         // 成功率（0-100）= 该渠道各模型 succ_ewma 均值×100，仅统计有真实上报的模型
	LatencyMs           any         // 平均延迟（毫秒）= 该渠道各模型 lat_ewma 均值，仅统计有真实上报的模型；仅展示用，不参与健康分计算
	StabilityScore      any         // 【已废弃】稳定性评分，健康体系重写后不再写入，保留列仅为兼容历史数据
	ConsecutiveFailures any         // 【已废弃】连续失败次数，已由 Redis 熔断器的滑动窗口计数取代（dispatch:v1:breaker:*），保留列仅为兼容历史数据
	HealthScore         any         // 综合健康度（0-100）= avg(succ_ewma)^α × 100，α 取路由策略 health.alpha（默认 2），与调度 healthFactor 同源；每 5 分钟由维护任务聚合落盘，另可由渠道测试/重置健康度即时触发；Redis 读失败或全部模型无真实上报时保留旧值不覆盖。调度决策不读此表，仅供管理后台展示
	CalculatedAt        *gtime.Time // 最近一次聚合落盘时间
	CreatedAt           *gtime.Time // 创建时间
	UpdatedAt           *gtime.Time // 更新时间
}
