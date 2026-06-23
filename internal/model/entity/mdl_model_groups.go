// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlModelGroups is the golang structure for table mdl_model_groups.
type MdlModelGroups struct {
	Id          int64       `json:"id"          orm:"id"          description:"主键ID"`                             // 主键ID
	Name        string      `json:"name"        orm:"name"        description:"分组名称（如\"全量模型\"、\"基础对话\"）"`         // 分组名称（如"全量模型"、"基础对话"）
	Code        string      `json:"code"        orm:"code"        description:"分组唯一标识（如 full_access、basic_chat）"` // 分组唯一标识（如 full_access、basic_chat）
	Description string      `json:"description" orm:"description" description:"分组描述"`                             // 分组描述
	Status      string      `json:"status"      orm:"status"      description:"状态：active（启用）/ disabled（禁用）"`      // 状态：active（启用）/ disabled（禁用）
	CreatedAt   *gtime.Time `json:"created_at"  orm:"created_at"  description:"创建时间"`                             // 创建时间
	UpdatedAt   *gtime.Time `json:"updated_at"  orm:"updated_at"  description:"更新时间"`                             // 更新时间
	IsDefault   bool        `json:"is_default"  orm:"is_default"  description:"是否为新租户默认模型组，注册时自动关联"`              // 是否为新租户默认模型组，注册时自动关联
}
