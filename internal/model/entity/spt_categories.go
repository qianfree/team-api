// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SptCategories is the golang structure for table spt_categories.
type SptCategories struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`          // 主键ID
	ParentId     int64       `json:"parent_id"     orm:"parent_id"     description:"父分类ID，0表示顶级分类"` // 父分类ID，0表示顶级分类
	Name         string      `json:"name"          orm:"name"          description:"分类名称"`          // 分类名称
	Slug         string      `json:"slug"          orm:"slug"          description:"URL 友好标识，唯一"`   // URL 友好标识，唯一
	Description  string      `json:"description"   orm:"description"   description:"分类描述"`          // 分类描述
	SortOrder    int         `json:"sort_order"    orm:"sort_order"    description:"排序序号，越小越靠前"`    // 排序序号，越小越靠前
	Icon         string      `json:"icon"          orm:"icon"          description:"图标名称"`          // 图标名称
	IsVisible    bool        `json:"is_visible"    orm:"is_visible"    description:"是否对外可见"`        // 是否对外可见
	ArticleCount int         `json:"article_count" orm:"article_count" description:"分类下文章数量（冗余计数）"` // 分类下文章数量（冗余计数）
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:""`              //
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:""`              //
}
