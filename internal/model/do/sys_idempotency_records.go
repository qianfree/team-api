// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysIdempotencyRecords is the golang structure of table sys_idempotency_records for DAO operations like Where/Data.
type SysIdempotencyRecords struct {
	g.Meta         `orm:"table:sys_idempotency_records, do:true"`
	Id             any         // 主键ID
	IdempotencyKey any         // 幂等键（来自请求头 Idempotency-Key）
	RequestHash    any         // 请求体哈希（SHA-256，用于校验请求一致性）
	ResponseBody   any         // 首次处理的响应体（幂等返回时复用）
	Status         any         // 状态：processing（处理中）/ completed（已完成）/ failed（失败）
	ExpiresAt      *gtime.Time // 过期时间（过期后记录可清理）
	CreatedAt      *gtime.Time // 创建时间
	UpdatedAt      *gtime.Time // 更新时间
}
