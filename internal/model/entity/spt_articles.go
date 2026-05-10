// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SptArticles is the golang structure for table spt_articles.
type SptArticles struct {
	Id          int64       `json:"id"           orm:"id"           description:"主键ID"`                 // 主键ID
	CategoryId  int64       `json:"category_id"  orm:"category_id"  description:"所属分类ID"`               // 所属分类ID
	Title       string      `json:"title"        orm:"title"        description:"文章标题"`                 // 文章标题
	Slug        string      `json:"slug"         orm:"slug"         description:"URL 友好标识，唯一"`          // URL 友好标识，唯一
	Content     string      `json:"content"      orm:"content"      description:"文章内容（Markdown）"`       // 文章内容（Markdown）
	Summary     string      `json:"summary"      orm:"summary"      description:"文章摘要"`                 // 文章摘要
	Status      string      `json:"status"       orm:"status"       description:"状态：draft / published"` // 状态：draft / published
	AuthorId    int64       `json:"author_id"    orm:"author_id"    description:"作者（管理员）ID"`            // 作者（管理员）ID
	ViewCount   int         `json:"view_count"   orm:"view_count"   description:"浏览次数"`                 // 浏览次数
	SortOrder   int         `json:"sort_order"   orm:"sort_order"   description:"排序序号，越小越靠前"`           // 排序序号，越小越靠前
	Keywords    string      `json:"keywords"     orm:"keywords"     description:"关键词（JSON 数组）"`         // 关键词（JSON 数组）
	PublishedAt *gtime.Time `json:"published_at" orm:"published_at" description:"发布时间"`                 // 发布时间
	CreatedAt   *gtime.Time `json:"created_at"   orm:"created_at"   description:""`                     //
	UpdatedAt   *gtime.Time `json:"updated_at"   orm:"updated_at"   description:""`                     //
}
