// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlModelGroups is the golang structure of table mdl_model_groups for DAO operations like Where/Data.
type MdlModelGroups struct {
	g.Meta      `orm:"table:mdl_model_groups, do:true"`
	Id          any         // 主键ID
	Name        any         // 分组名称（如"全量模型"、"基础对话"）
	Code        any         // 分组唯一标识（如 full_access、basic_chat）
	Description any         // 分组描述
	Status      any         // 状态：active（启用）/ disabled（禁用）
	IsDefault   any         // 是否为新租户默认模型组，注册时自动关联
	CreatedAt   *gtime.Time // 创建时间
	UpdatedAt   *gtime.Time // 更新时间
}
