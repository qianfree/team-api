// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// JsPluginVersions is the golang structure of table js_plugin_versions for DAO operations like Where/Data.
type JsPluginVersions struct {
	g.Meta     `orm:"table:js_plugin_versions, do:true"`
	Id         any         //
	PluginKey  any         // 插件标识（meta.key）
	Version    any         //
	Source     any         //
	SourceHash any         //
	Compiled   any         // esbuild IIFE 编译产物（激活时编译缓存，可空）
	Manifest   any         // 解析校验后的 meta 清单
	Enabled    any         // 插件级开关（active 版本行上生效）
	Active     any         // 激活版本（每 plugin_key 至多一行）
	Remark     any         //
	CreatedAt  *gtime.Time //
	UpdatedAt  *gtime.Time //
}
