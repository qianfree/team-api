// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminRolePerms is the golang structure for table sys_admin_role_perms.
type SysAdminRolePerms struct {
	Id              int64       `json:"id"               orm:"id"               description:"主键ID"`                                // 主键ID
	AdminUserId     int64       `json:"admin_user_id"    orm:"admin_user_id"    description:"关联的管理员用户ID"`                          // 关联的管理员用户ID
	PermissionPoint string      `json:"permission_point" orm:"permission_point" description:"权限点标识（如 tenant:create、channel:edit）"` // 权限点标识（如 tenant:create、channel:edit）
	CreatedAt       *gtime.Time `json:"created_at"       orm:"created_at"       description:"创建时间"`                                // 创建时间
	UpdatedAt       *gtime.Time `json:"updated_at"       orm:"updated_at"       description:"更新时间"`                                // 更新时间
}
