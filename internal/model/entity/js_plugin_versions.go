// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// JsPluginVersions is the golang structure for table js_plugin_versions.
type JsPluginVersions struct {
	Id         int64       `json:"id"          orm:"id"          description:""`                              //
	PluginKey  string      `json:"plugin_key"  orm:"plugin_key"  description:"插件标识（meta.key）"`                // 插件标识（meta.key）
	Version    string      `json:"version"     orm:"version"     description:""`                              //
	Source     string      `json:"source"      orm:"source"      description:""`                              //
	SourceHash string      `json:"source_hash" orm:"source_hash" description:""`                              //
	Compiled   string      `json:"compiled"    orm:"compiled"    description:"esbuild IIFE 编译产物（激活时编译缓存，可空）"` // esbuild IIFE 编译产物（激活时编译缓存，可空）
	Manifest   string      `json:"manifest"    orm:"manifest"    description:"解析校验后的 meta 清单"`                // 解析校验后的 meta 清单
	Enabled    bool        `json:"enabled"     orm:"enabled"     description:"插件级开关（active 版本行上生效）"`          // 插件级开关（active 版本行上生效）
	Active     bool        `json:"active"      orm:"active"      description:"激活版本（每 plugin_key 至多一行）"`       // 激活版本（每 plugin_key 至多一行）
	Remark     string      `json:"remark"      orm:"remark"      description:""`                              //
	CreatedAt  *gtime.Time `json:"created_at"  orm:"created_at"  description:""`                              //
	UpdatedAt  *gtime.Time `json:"updated_at"  orm:"updated_at"  description:""`                              //
}
