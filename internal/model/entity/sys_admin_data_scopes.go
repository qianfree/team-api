// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminDataScopes is the golang structure for table sys_admin_data_scopes.
type SysAdminDataScopes struct {
	Id          int64       `json:"id"            orm:"id"            description:"主键ID"`                                          // 主键ID
	AdminUserId int64       `json:"admin_user_id" orm:"admin_user_id" description:"关联的管理员用户ID"`                                    // 关联的管理员用户ID
	ScopeType   string      `json:"scope_type"    orm:"scope_type"    description:"范围类型：all（全部）/ tenant_group（租户组）/ tenant（指定租户）"` // 范围类型：all（全部）/ tenant_group（租户组）/ tenant（指定租户）
	ScopeValue  string      `json:"scope_value"   orm:"scope_value"   description:"范围值（tenant_group时为组名，tenant时为租户ID列表，逗号分隔）"`     // 范围值（tenant_group时为组名，tenant时为租户ID列表，逗号分隔）
	CreatedAt   *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                                          // 创建时间
	UpdatedAt   *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                                          // 更新时间
}
