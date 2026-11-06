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
	Id         any         // 主键
	PluginKey  any         // 插件标识（meta.key）
	Version    any         // 插件版本（semver，key+version 唯一）
	Source     any         // plugin.js 源码原文（上传/内置 embed）
	SourceHash any         // 源码 SHA-256（同版本不可改内容的校验依据）
	Compiled   any         // esbuild IIFE 编译产物（激活时编译缓存，可空）
	Manifest   any         // 解析校验后的 meta 清单
	Origin     any         // 版本来源：builtin=内置（go:embed 随二进制，不可删除）/ upload=管理端上传
	Icon       any         // 插件图标（data URI，仅上传版本；内置版本用 manifest.icon）
	Enabled    any         // 插件级开关（active 版本行上生效）
	Active     any         // 激活版本（每 plugin_key 至多一行）
	Remark     any         // 管理端备注
	CreatedAt  *gtime.Time // 创建时间
	UpdatedAt  *gtime.Time // 更新时间
}
