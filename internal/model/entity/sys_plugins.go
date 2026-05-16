// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysPlugins is the golang structure for table sys_plugins.
type SysPlugins struct {
	Id        int64       `json:"id"         orm:"id"         description:""`                                                                           //
	Name      string      `json:"name"       orm:"name"       description:"插件唯一标识，如 email-report"`                                                      // 插件唯一标识，如 email-report
	Label     string      `json:"label"      orm:"label"      description:"显示名称"`                                                                       // 显示名称
	Version   string      `json:"version"    orm:"version"    description:"当前安装版本"`                                                                     // 当前安装版本
	Status    string      `json:"status"     orm:"status"     description:"状态：registered=已注册, installed=已安装, enabled=已启用, disabled=已禁用, error=异常"`      // 状态：registered=已注册, installed=已安装, enabled=已启用, disabled=已禁用, error=异常
	Category  string      `json:"category"   orm:"category"   description:"分类：relay=代理扩展, middleware=中间件, billing=计费, notification=通知, extension=通用扩展"` // 分类：relay=代理扩展, middleware=中间件, billing=计费, notification=通知, extension=通用扩展
	Config    string      `json:"config"     orm:"config"     description:"插件全局配置（JSON）"`                                                               // 插件全局配置（JSON）
	ErrorMsg  string      `json:"error_msg"  orm:"error_msg"  description:"异常信息"`                                                                       // 异常信息
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:""`                                                                           //
	UpdatedAt *gtime.Time `json:"updated_at" orm:"updated_at" description:""`                                                                           //
}
