// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysOptions is the golang structure of table sys_options for DAO operations like Where/Data.
type SysOptions struct {
	g.Meta      `orm:"table:sys_options, do:true"`
	Id          any         // 主键ID
	Key         any         // 配置键（唯一标识，如 site_name、register_enabled）
	Value       any         // 配置值
	Description any         // 配置说明
	Category    any         // 配置分类（如 general、security、email、payment）
	IsPublic    any         // 是否公开（前端可直接获取，如站点名称、注册开关）
	CreatedAt   *gtime.Time // 创建时间
	UpdatedAt   *gtime.Time // 更新时间
}
