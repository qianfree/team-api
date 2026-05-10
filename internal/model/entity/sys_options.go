// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysOptions is the golang structure for table sys_options.
type SysOptions struct {
	Id          int64       `json:"id"          orm:"id"          description:"主键ID"`                                   // 主键ID
	Key         string      `json:"key"         orm:"key"         description:"配置键（唯一标识，如 site_name、register_enabled）"` // 配置键（唯一标识，如 site_name、register_enabled）
	Value       string      `json:"value"       orm:"value"       description:"配置值"`                                    // 配置值
	Description string      `json:"description" orm:"description" description:"配置说明"`                                   // 配置说明
	Category    string      `json:"category"    orm:"category"    description:"配置分类（如 general、security、email、payment）"` // 配置分类（如 general、security、email、payment）
	IsPublic    bool        `json:"is_public"   orm:"is_public"   description:"是否公开（前端可直接获取，如站点名称、注册开关）"`               // 是否公开（前端可直接获取，如站点名称、注册开关）
	CreatedAt   *gtime.Time `json:"created_at"  orm:"created_at"  description:"创建时间"`                                   // 创建时间
	UpdatedAt   *gtime.Time `json:"updated_at"  orm:"updated_at"  description:"更新时间"`                                   // 更新时间
}
