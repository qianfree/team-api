// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminRoles is the golang structure of table sys_admin_roles for DAO operations like Where/Data.
type SysAdminRoles struct {
	g.Meta      `orm:"table:sys_admin_roles, do:true"`
	Id          any         // 主键ID
	Code        any         // 角色标识（小写字母/数字/下划线，创建后不可修改，是权限缓存与审计日志的稳定标识；保留字 super_admin 不可占用）
	Name        any         // 角色显示名称（可修改）
	Description any         // 角色说明，在分配界面展示
	IsBuiltin   any         // 是否系统预置：仅用于标识来源与支撑「恢复默认权限」，不构成删除保护（预置角色同样可改可删）
	IsEnabled   any         // 是否启用：禁用后该角色的权限对全部关联用户立即失效，无需解除关联
	Sort        any         // 排序权重（升序）
	CreatedAt   *gtime.Time // 创建时间
	UpdatedAt   *gtime.Time // 更新时间
}
