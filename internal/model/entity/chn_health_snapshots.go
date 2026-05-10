// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnHealthSnapshots is the golang structure for table chn_health_snapshots.
type ChnHealthSnapshots struct {
	Id                  int64       `json:"id"                   orm:"id"                   description:"主键ID"`         // 主键ID
	ChannelId           int64       `json:"channel_id"           orm:"channel_id"           description:"关联渠道ID"`       // 关联渠道ID
	HealthScore         float64     `json:"health_score"         orm:"health_score"         description:"综合健康度（0-100）"` // 综合健康度（0-100）
	SuccessRate         float64     `json:"success_rate"         orm:"success_rate"         description:"请求成功率（0-100）"` // 请求成功率（0-100）
	LatencyMs           float64     `json:"latency_ms"           orm:"latency_ms"           description:"平均延迟（毫秒）"`     // 平均延迟（毫秒）
	StabilityScore      float64     `json:"stability_score"      orm:"stability_score"      description:"稳定性评分（0-100）"` // 稳定性评分（0-100）
	ConsecutiveFailures int         `json:"consecutive_failures" orm:"consecutive_failures" description:"连续失败次数"`       // 连续失败次数
	SnapshotAt          *gtime.Time `json:"snapshot_at"          orm:"snapshot_at"          description:"快照时间"`         // 快照时间
}
