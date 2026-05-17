// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TntTenantPlugins is the golang structure for table tnt_tenant_plugins.
type TntTenantPlugins struct {
	Id         int64       `json:"id"          orm:"id"          description:""`                        //
	TenantId   int64       `json:"tenant_id"   orm:"tenant_id"   description:""`                        //
	PluginName string      `json:"plugin_name" orm:"plugin_name" description:"插件标识"`                    // 插件标识
	Enabled    bool        `json:"enabled"     orm:"enabled"     description:"是否启用"`                    // 是否启用
	Config     string      `json:"config"      orm:"config"      description:"租户级配置覆盖（JSON），优先级高于全局配置"` // 租户级配置覆盖（JSON），优先级高于全局配置
	CreatedAt  *gtime.Time `json:"created_at"  orm:"created_at"  description:""`                        //
	UpdatedAt  *gtime.Time `json:"updated_at"  orm:"updated_at"  description:""`                        //
}
