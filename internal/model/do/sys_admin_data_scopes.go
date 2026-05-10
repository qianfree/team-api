// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminDataScopes is the golang structure of table sys_admin_data_scopes for DAO operations like Where/Data.
type SysAdminDataScopes struct {
	g.Meta      `orm:"table:sys_admin_data_scopes, do:true"`
	Id          any         // 主键ID
	AdminUserId any         // 关联的管理员用户ID
	ScopeType   any         // 范围类型：all（全部）/ tenant_group（租户组）/ tenant（指定租户）
	ScopeValue  any         // 范围值（tenant_group时为组名，tenant时为租户ID列表，逗号分隔）
	CreatedAt   *gtime.Time // 创建时间
	UpdatedAt   *gtime.Time // 更新时间
}
