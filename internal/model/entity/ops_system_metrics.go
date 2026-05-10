// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OpsSystemMetrics is the golang structure for table ops_system_metrics.
type OpsSystemMetrics struct {
	Id          int64       `json:"id"           orm:"id"           description:"主键ID"`                                                    // 主键ID
	MetricType  string      `json:"metric_type"  orm:"metric_type"  description:"指标类型：cpu/memory/disk/network/runtime/db_pool/redis_pool"` // 指标类型：cpu/memory/disk/network/runtime/db_pool/redis_pool
	MetricData  string      `json:"metric_data"  orm:"metric_data"  description:"指标数据（JSONB，结构因类型而异）"`                                     // 指标数据（JSONB，结构因类型而异）
	CollectedAt *gtime.Time `json:"collected_at" orm:"collected_at" description:"采集时间"`                                                    // 采集时间
}
