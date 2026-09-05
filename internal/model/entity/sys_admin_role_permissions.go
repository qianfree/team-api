// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminRolePermissions is the golang structure for table sys_admin_role_permissions.
type SysAdminRolePermissions struct {
	Id              int64       `json:"id"               orm:"id"               description:"主键ID"`                                            // 主键ID
	RoleId          int64       `json:"role_id"          orm:"role_id"          description:"角色ID（逻辑关联 sys_admin_roles.id，无外键，删除角色时由业务层级联清理）"` // 角色ID（逻辑关联 sys_admin_roles.id，无外键，删除角色时由业务层级联清理）
	PermissionPoint string      `json:"permission_point" orm:"permission_point" description:"权限点标识（如 tenant:create、channel:edit）"`             // 权限点标识（如 tenant:create、channel:edit）
	CreatedAt       *gtime.Time `json:"created_at"       orm:"created_at"       description:"创建时间"`                                            // 创建时间
	UpdatedAt       *gtime.Time `json:"updated_at"       orm:"updated_at"       description:"更新时间"`                                            // 更新时间
}
