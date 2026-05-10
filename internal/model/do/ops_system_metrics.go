// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OpsSystemMetrics is the golang structure of table ops_system_metrics for DAO operations like Where/Data.
type OpsSystemMetrics struct {
	g.Meta      `orm:"table:ops_system_metrics, do:true"`
	Id          any         // 主键ID
	MetricType  any         // 指标类型：cpu/memory/disk/network/runtime/db_pool/redis_pool
	MetricData  any         // 指标数据（JSONB，结构因类型而异）
	CollectedAt *gtime.Time // 采集时间
}
