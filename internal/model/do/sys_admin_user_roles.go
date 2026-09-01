// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminUserRoles is the golang structure of table sys_admin_user_roles for DAO operations like Where/Data.
type SysAdminUserRoles struct {
	g.Meta      `orm:"table:sys_admin_user_roles, do:true"`
	Id          any         // 主键ID
	AdminUserId any         // 管理员用户ID（逻辑关联 sys_admin_users.id，无外键）
	RoleId      any         // 角色ID（逻辑关联 sys_admin_roles.id，无外键）
	CreatedAt   *gtime.Time // 创建时间
	UpdatedAt   *gtime.Time // 更新时间
}
