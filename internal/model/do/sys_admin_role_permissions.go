// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminRolePermissions is the golang structure of table sys_admin_role_permissions for DAO operations like Where/Data.
type SysAdminRolePermissions struct {
	g.Meta          `orm:"table:sys_admin_role_permissions, do:true"`
	Id              any         // 主键ID
	RoleId          any         // 角色ID（逻辑关联 sys_admin_roles.id，无外键，删除角色时由业务层级联清理）
	PermissionPoint any         // 权限点标识（如 tenant:create、channel:edit）
	CreatedAt       *gtime.Time // 创建时间
	UpdatedAt       *gtime.Time // 更新时间
}
