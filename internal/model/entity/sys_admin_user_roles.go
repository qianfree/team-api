// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminUserRoles is the golang structure for table sys_admin_user_roles.
type SysAdminUserRoles struct {
	Id          int64       `json:"id"            orm:"id"            description:"主键ID"`                                 // 主键ID
	AdminUserId int64       `json:"admin_user_id" orm:"admin_user_id" description:"管理员用户ID（逻辑关联 sys_admin_users.id，无外键）"` // 管理员用户ID（逻辑关联 sys_admin_users.id，无外键）
	RoleId      int64       `json:"role_id"       orm:"role_id"       description:"角色ID（逻辑关联 sys_admin_roles.id，无外键）"`    // 角色ID（逻辑关联 sys_admin_roles.id，无外键）
	CreatedAt   *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                                 // 创建时间
	UpdatedAt   *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                                 // 更新时间
}
