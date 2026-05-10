// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnHealthSnapshots is the golang structure of table chn_health_snapshots for DAO operations like Where/Data.
type ChnHealthSnapshots struct {
	g.Meta              `orm:"table:chn_health_snapshots, do:true"`
	Id                  any         // 主键ID
	ChannelId           any         // 关联渠道ID
	HealthScore         any         // 综合健康度（0-100）
	SuccessRate         any         // 请求成功率（0-100）
	LatencyMs           any         // 平均延迟（毫秒）
	StabilityScore      any         // 稳定性评分（0-100）
	ConsecutiveFailures any         // 连续失败次数
	SnapshotAt          *gtime.Time // 快照时间
}
