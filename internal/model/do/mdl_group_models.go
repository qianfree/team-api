// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlGroupModels is the golang structure of table mdl_group_models for DAO operations like Where/Data.
type MdlGroupModels struct {
	g.Meta    `orm:"table:mdl_group_models, do:true"`
	Id        any         // 主键ID
	GroupId   any         // 分组ID（关联 mdl_model_groups.id）
	ModelId   any         // 模型ID（关联 mdl_models.id）
	CreatedAt *gtime.Time // 创建时间
	UpdatedAt *gtime.Time // 更新时间
}
