// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// AudOperationLogs is the golang structure for table aud_operation_logs.
type AudOperationLogs struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`                                               // 主键ID
	TenantId     int64       `json:"tenant_id"     orm:"tenant_id"     description:"租户ID（管理后台操作时为 NULL）"`                                // 租户ID（管理后台操作时为 NULL）
	UserId       int64       `json:"user_id"       orm:"user_id"       description:"操作者用户ID"`                                            // 操作者用户ID
	UserType     string      `json:"user_type"     orm:"user_type"     description:"操作者类型：admin（管理后台）/ tenant（租户控制台）"`                   // 操作者类型：admin（管理后台）/ tenant（租户控制台）
	Action       string      `json:"action"        orm:"action"        description:"操作动作（如 create_tenant、update_channel、delete_model）"`  // 操作动作（如 create_tenant、update_channel、delete_model）
	ResourceType string      `json:"resource_type" orm:"resource_type" description:"操作资源类型：tenant / channel / model / user / api_key 等"` // 操作资源类型：tenant / channel / model / user / api_key 等
	ResourceId   int64       `json:"resource_id"   orm:"resource_id"   description:"操作资源ID"`                                             // 操作资源ID
	Detail       string      `json:"detail"        orm:"detail"        description:"操作详情（JSONB：变更前后的字段差异等）"`                             // 操作详情（JSONB：变更前后的字段差异等）
	IpAddress    string      `json:"ip_address"    orm:"ip_address"    description:"操作者 IP 地址"`                                          // 操作者 IP 地址
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                                               // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                                               // 更新时间
	ChangesJson  string      `json:"changes_json"  orm:"changes_json"  description:"变更前后数据对比（JSON diff）"`                                // 变更前后数据对比（JSON diff）
}
