// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysIdempotencyRecords is the golang structure for table sys_idempotency_records.
type SysIdempotencyRecords struct {
	Id             int64       `json:"id"              orm:"id"              description:"主键ID"`                                           // 主键ID
	IdempotencyKey string      `json:"idempotency_key" orm:"idempotency_key" description:"幂等键（来自请求头 Idempotency-Key）"`                     // 幂等键（来自请求头 Idempotency-Key）
	RequestHash    string      `json:"request_hash"    orm:"request_hash"    description:"请求体哈希（SHA-256，用于校验请求一致性）"`                       // 请求体哈希（SHA-256，用于校验请求一致性）
	ResponseBody   string      `json:"response_body"   orm:"response_body"   description:"首次处理的响应体（幂等返回时复用）"`                              // 首次处理的响应体（幂等返回时复用）
	Status         string      `json:"status"          orm:"status"          description:"状态：processing（处理中）/ completed（已完成）/ failed（失败）"` // 状态：processing（处理中）/ completed（已完成）/ failed（失败）
	ExpiresAt      *gtime.Time `json:"expires_at"      orm:"expires_at"      description:"过期时间（过期后记录可清理）"`                                 // 过期时间（过期后记录可清理）
	CreatedAt      *gtime.Time `json:"created_at"      orm:"created_at"      description:"创建时间"`                                           // 创建时间
	UpdatedAt      *gtime.Time `json:"updated_at"      orm:"updated_at"      description:"更新时间"`                                           // 更新时间
}
