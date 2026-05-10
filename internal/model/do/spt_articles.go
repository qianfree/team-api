// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SptArticles is the golang structure of table spt_articles for DAO operations like Where/Data.
type SptArticles struct {
	g.Meta      `orm:"table:spt_articles, do:true"`
	Id          any         // 主键ID
	CategoryId  any         // 所属分类ID
	Title       any         // 文章标题
	Slug        any         // URL 友好标识，唯一
	Content     any         // 文章内容（Markdown）
	Summary     any         // 文章摘要
	Status      any         // 状态：draft / published
	AuthorId    any         // 作者（管理员）ID
	ViewCount   any         // 浏览次数
	SortOrder   any         // 排序序号，越小越靠前
	Keywords    any         // 关键词（JSON 数组）
	PublishedAt *gtime.Time // 发布时间
	CreatedAt   *gtime.Time //
	UpdatedAt   *gtime.Time //
}
