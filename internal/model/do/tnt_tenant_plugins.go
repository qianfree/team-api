// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TntTenantPlugins is the golang structure of table tnt_tenant_plugins for DAO operations like Where/Data.
type TntTenantPlugins struct {
	g.Meta     `orm:"table:tnt_tenant_plugins, do:true"`
	Id         any         //
	TenantId   any         //
	PluginName any         // 插件标识
	Enabled    any         // 是否启用
	Config     any         // 租户级配置覆盖（JSON），优先级高于全局配置
	CreatedAt  *gtime.Time //
	UpdatedAt  *gtime.Time //
}
