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
	SuccessRate         any         // 请求成功率（0-100）
	LatencyMs           any         // 平均延迟（毫秒）
	StabilityScore      any         // 稳定性评分（0-100，基于延迟波动计算）
	ConsecutiveFailures any         // 连续失败次数（成功后归零）
	HealthScore         any         // 综合健康度（0-100）= 成功率×0.40 + 延迟分×0.25 + 稳定性×0.20 + 连续失败分×0.15
	CalculatedAt        *gtime.Time // 最近一次计算时间
	CreatedAt           *gtime.Time // 创建时间
	UpdatedAt           *gtime.Time // 更新时间
}
