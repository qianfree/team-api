// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnHealthScores is the golang structure for table chn_health_scores.
type ChnHealthScores struct {
	Id                  int64       `json:"id"                   orm:"id"                   description:"主键ID"`                                                      // 主键ID
	ChannelId           int64       `json:"channel_id"           orm:"channel_id"           description:"关联渠道ID"`                                                    // 关联渠道ID
	SuccessRate         float64     `json:"success_rate"         orm:"success_rate"         description:"请求成功率（0-100）"`                                              // 请求成功率（0-100）
	LatencyMs           float64     `json:"latency_ms"           orm:"latency_ms"           description:"平均延迟（毫秒）"`                                                  // 平均延迟（毫秒）
	StabilityScore      float64     `json:"stability_score"      orm:"stability_score"      description:"稳定性评分（0-100，基于延迟波动计算）"`                                     // 稳定性评分（0-100，基于延迟波动计算）
	ConsecutiveFailures int         `json:"consecutive_failures" orm:"consecutive_failures" description:"连续失败次数（成功后归零）"`                                             // 连续失败次数（成功后归零）
	HealthScore         float64     `json:"health_score"         orm:"health_score"         description:"综合健康度（0-100）= 成功率×0.40 + 延迟分×0.25 + 稳定性×0.20 + 连续失败分×0.15"` // 综合健康度（0-100）= 成功率×0.40 + 延迟分×0.25 + 稳定性×0.20 + 连续失败分×0.15
	CalculatedAt        *gtime.Time `json:"calculated_at"        orm:"calculated_at"        description:"最近一次计算时间"`                                                  // 最近一次计算时间
	CreatedAt           *gtime.Time `json:"created_at"           orm:"created_at"           description:"创建时间"`                                                      // 创建时间
	UpdatedAt           *gtime.Time `json:"updated_at"           orm:"updated_at"           description:"更新时间"`                                                      // 更新时间
}
