// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminRolePerms is the golang structure of table sys_admin_role_perms for DAO operations like Where/Data.
type SysAdminRolePerms struct {
	g.Meta          `orm:"table:sys_admin_role_perms, do:true"`
	Id              any         // 主键ID
	AdminUserId     any         // 关联的管理员用户ID
	PermissionPoint any         // 权限点标识（如 tenant:create、channel:edit）
	CreatedAt       *gtime.Time // 创建时间
	UpdatedAt       *gtime.Time // 更新时间
}
