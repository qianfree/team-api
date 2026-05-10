// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// AudSensitiveAccessLogs is the golang structure for table aud_sensitive_access_logs.
type AudSensitiveAccessLogs struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`                             // 主键ID
	UserId       int64       `json:"user_id"       orm:"user_id"       description:"访问者用户ID"`                          // 访问者用户ID
	UserType     string      `json:"user_type"     orm:"user_type"     description:"访问者类型：admin（管理后台）/ tenant（租户控制台）"` // 访问者类型：admin（管理后台）/ tenant（租户控制台）
	ResourceType string      `json:"resource_type" orm:"resource_type" description:"资源类型（如 api_key、channel、wallet 等）"` // 资源类型（如 api_key、channel、wallet 等）
	ResourceId   int64       `json:"resource_id"   orm:"resource_id"   description:"资源ID"`                             // 资源ID
	Action       string      `json:"action"        orm:"action"        description:"访问动作（如 view、export、download）"`     // 访问动作（如 view、export、download）
	Reason       string      `json:"reason"        orm:"reason"        description:"访问原因（查看敏感数据时需填写）"`                 // 访问原因（查看敏感数据时需填写）
	IpAddress    string      `json:"ip_address"    orm:"ip_address"    description:"访问者 IP 地址"`                        // 访问者 IP 地址
	UserAgent    string      `json:"user_agent"    orm:"user_agent"    description:"访问者 User-Agent"`                   // 访问者 User-Agent
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                             // 创建时间
}
