// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysPlugins is the golang structure of table sys_plugins for DAO operations like Where/Data.
type SysPlugins struct {
	g.Meta    `orm:"table:sys_plugins, do:true"`
	Id        any         //
	Name      any         // 插件唯一标识，如 email-report
	Label     any         // 显示名称
	Version   any         // 当前安装版本
	Status    any         // 状态：registered=已注册, installed=已安装, enabled=已启用, disabled=已禁用, error=异常
	Category  any         // 分类：relay=代理扩展, middleware=中间件, billing=计费, notification=通知, extension=通用扩展
	Config    any         // 插件全局配置（JSON）
	ErrorMsg  any         // 异常信息
	CreatedAt *gtime.Time //
	UpdatedAt *gtime.Time //
}
