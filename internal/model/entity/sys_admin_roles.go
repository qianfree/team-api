// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminRoles is the golang structure for table sys_admin_roles.
type SysAdminRoles struct {
	Id          int64       `json:"id"          orm:"id"          description:"主键ID"`                                                           // 主键ID
	Code        string      `json:"code"        orm:"code"        description:"角色标识（小写字母/数字/下划线，创建后不可修改，是权限缓存与审计日志的稳定标识；保留字 super_admin 不可占用）"` // 角色标识（小写字母/数字/下划线，创建后不可修改，是权限缓存与审计日志的稳定标识；保留字 super_admin 不可占用）
	Name        string      `json:"name"        orm:"name"        description:"角色显示名称（可修改）"`                                                    // 角色显示名称（可修改）
	Description string      `json:"description" orm:"description" description:"角色说明，在分配界面展示"`                                                   // 角色说明，在分配界面展示
	IsBuiltin   bool        `json:"is_builtin"  orm:"is_builtin"  description:"是否系统预置：仅用于标识来源与支撑「恢复默认权限」，不构成删除保护（预置角色同样可改可删）"`                  // 是否系统预置：仅用于标识来源与支撑「恢复默认权限」，不构成删除保护（预置角色同样可改可删）
	IsEnabled   bool        `json:"is_enabled"  orm:"is_enabled"  description:"是否启用：禁用后该角色的权限对全部关联用户立即失效，无需解除关联"`                               // 是否启用：禁用后该角色的权限对全部关联用户立即失效，无需解除关联
	Sort        int         `json:"sort"        orm:"sort"        description:"排序权重（升序）"`                                                       // 排序权重（升序）
	CreatedAt   *gtime.Time `json:"created_at"  orm:"created_at"  description:"创建时间"`                                                           // 创建时间
	UpdatedAt   *gtime.Time `json:"updated_at"  orm:"updated_at"  description:"更新时间"`                                                           // 更新时间
}
