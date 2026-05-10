// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SptCategories is the golang structure of table spt_categories for DAO operations like Where/Data.
type SptCategories struct {
	g.Meta       `orm:"table:spt_categories, do:true"`
	Id           any         // 主键ID
	ParentId     any         // 父分类ID，0表示顶级分类
	Name         any         // 分类名称
	Slug         any         // URL 友好标识，唯一
	Description  any         // 分类描述
	SortOrder    any         // 排序序号，越小越靠前
	Icon         any         // 图标名称
	IsVisible    any         // 是否对外可见
	ArticleCount any         // 分类下文章数量（冗余计数）
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
}
