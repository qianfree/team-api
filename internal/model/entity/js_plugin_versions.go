// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// JsPluginVersions is the golang structure for table js_plugin_versions.
type JsPluginVersions struct {
	Id         int64       `json:"id"          orm:"id"          description:"主键"`                                                // 主键
	PluginKey  string      `json:"plugin_key"  orm:"plugin_key"  description:"插件标识（meta.key）"`                                    // 插件标识（meta.key）
	Version    string      `json:"version"     orm:"version"     description:"插件版本（semver，key+version 唯一）"`                       // 插件版本（semver，key+version 唯一）
	Source     string      `json:"source"      orm:"source"      description:"plugin.js 源码原文（上传/内置 embed）"`                       // plugin.js 源码原文（上传/内置 embed）
	SourceHash string      `json:"source_hash" orm:"source_hash" description:"源码 SHA-256（同版本不可改内容的校验依据）"`                         // 源码 SHA-256（同版本不可改内容的校验依据）
	Compiled   string      `json:"compiled"    orm:"compiled"    description:"esbuild IIFE 编译产物（激活时编译缓存，可空）"`                     // esbuild IIFE 编译产物（激活时编译缓存，可空）
	Manifest   string      `json:"manifest"    orm:"manifest"    description:"解析校验后的 meta 清单"`                                    // 解析校验后的 meta 清单
	Origin     string      `json:"origin"      orm:"origin"      description:"版本来源：builtin=内置（go:embed 随二进制，不可删除）/ upload=管理端上传"` // 版本来源：builtin=内置（go:embed 随二进制，不可删除）/ upload=管理端上传
	Icon       string      `json:"icon"        orm:"icon"        description:"插件图标（data URI，仅上传版本；内置版本用 manifest.icon）"`          // 插件图标（data URI，仅上传版本；内置版本用 manifest.icon）
	Enabled    bool        `json:"enabled"     orm:"enabled"     description:"插件级开关（active 版本行上生效）"`                              // 插件级开关（active 版本行上生效）
	Active     bool        `json:"active"      orm:"active"      description:"激活版本（每 plugin_key 至多一行）"`                           // 激活版本（每 plugin_key 至多一行）
	Remark     string      `json:"remark"      orm:"remark"      description:"管理端备注"`                                             // 管理端备注
	CreatedAt  *gtime.Time `json:"created_at"  orm:"created_at"  description:"创建时间"`                                              // 创建时间
	UpdatedAt  *gtime.Time `json:"updated_at"  orm:"updated_at"  description:"更新时间"`                                              // 更新时间
}
