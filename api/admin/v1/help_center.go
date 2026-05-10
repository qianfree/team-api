package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 帮助中心 - 分类（管理后台） ===

type HelpCategoryCreateReq struct {
	g.Meta      `path:"/help-categories" method:"post" mime:"json" tags:"管理后台-帮助中心" summary:"创建帮助分类"`
	ParentId    int64  `json:"parent_id" d:"0" dc:"父分类ID，0为顶级"`
	Name        string `json:"name" v:"required|length:1,100" dc:"分类名称"`
	Slug        string `json:"slug" v:"required|length:1,100" dc:"URL友好标识"`
	Description string `json:"description" dc:"分类描述"`
	SortOrder   int    `json:"sort_order" d:"0" dc:"排序序号"`
	Icon        string `json:"icon" dc:"图标名称"`
	IsVisible   *bool  `json:"is_visible" d:"true" dc:"是否对外可见"`
}

type HelpCategoryCreateRes struct {
	Id int64 `json:"id"`
}

type HelpCategoryUpdateReq struct {
	g.Meta      `path:"/help-categories/{id}" method:"put" mime:"json" tags:"管理后台-帮助中心" summary:"更新帮助分类"`
	Id          int64  `json:"id" in:"path" v:"required|min:1"`
	ParentId    int64  `json:"parent_id" d:"0" dc:"父分类ID，0为顶级"`
	Name        string `json:"name" v:"required|length:1,100" dc:"分类名称"`
	Slug        string `json:"slug" v:"required|length:1,100" dc:"URL友好标识"`
	Description string `json:"description" dc:"分类描述"`
	SortOrder   int    `json:"sort_order" d:"0" dc:"排序序号"`
	Icon        string `json:"icon" dc:"图标名称"`
	IsVisible   *bool  `json:"is_visible" dc:"是否对外可见"`
}

type HelpCategoryUpdateRes struct{}

type HelpCategoryDeleteReq struct {
	g.Meta `path:"/help-categories/{id}" method:"delete" mime:"json" tags:"管理后台-帮助中心" summary:"删除帮助分类"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type HelpCategoryDeleteRes struct{}

type HelpCategoryListReq struct {
	g.Meta   `path:"/help-categories" method:"get" mime:"json" tags:"管理后台-帮助中心" summary:"帮助分类列表"`
	ParentId int64 `json:"parent_id" in:"query" d:"-1" dc:"父分类ID，-1表示全部"`
	Page     int   `json:"page" in:"query" d:"1"`
	PageSize int   `json:"page_size" in:"query" d:"20"`
}

type HelpCategoryItem struct {
	Id           int64       `json:"id"`
	ParentId     int64       `json:"parent_id"`
	Name         string      `json:"name"`
	Slug         string      `json:"slug"`
	Description  string      `json:"description"`
	SortOrder    int         `json:"sort_order"`
	Icon         string      `json:"icon"`
	IsVisible    bool        `json:"is_visible"`
	ArticleCount int         `json:"article_count"`
	CreatedAt    *gtime.Time `json:"created_at"`
	UpdatedAt    *gtime.Time `json:"updated_at"`
}

type HelpCategoryListRes struct {
	List     []*HelpCategoryItem `json:"list"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// === 帮助中心 - 文章（管理后台） ===

type HelpArticleCreateReq struct {
	g.Meta     `path:"/help-articles" method:"post" mime:"json" tags:"管理后台-帮助中心" summary:"创建帮助文章"`
	CategoryId int64    `json:"category_id" v:"required|min:1" dc:"所属分类ID"`
	Title      string   `json:"title" v:"required|length:1,200" dc:"文章标题"`
	Slug       string   `json:"slug" v:"required|length:1,200" dc:"URL友好标识"`
	Content    string   `json:"content" v:"required" dc:"文章内容（Markdown）"`
	Summary    string   `json:"summary" dc:"文章摘要"`
	Status     string   `json:"status" d:"draft" v:"in:draft,published" dc:"状态"`
	SortOrder  int      `json:"sort_order" d:"0" dc:"排序序号"`
	Keywords   []string `json:"keywords" dc:"关键词列表"`
}

type HelpArticleCreateRes struct {
	Id int64 `json:"id"`
}

type HelpArticleUpdateReq struct {
	g.Meta     `path:"/help-articles/{id}" method:"put" mime:"json" tags:"管理后台-帮助中心" summary:"更新帮助文章"`
	Id         int64    `json:"id" in:"path" v:"required|min:1"`
	CategoryId int64    `json:"category_id" v:"required|min:1" dc:"所属分类ID"`
	Title      string   `json:"title" v:"required|length:1,200" dc:"文章标题"`
	Slug       string   `json:"slug" v:"required|length:1,200" dc:"URL友好标识"`
	Content    string   `json:"content" dc:"文章内容（Markdown），不传则不更新"`
	Summary    string   `json:"summary" dc:"文章摘要"`
	Status     string   `json:"status" v:"in:draft,published" dc:"状态"`
	SortOrder  int      `json:"sort_order" d:"0" dc:"排序序号"`
	Keywords   []string `json:"keywords" dc:"关键词列表"`
}

type HelpArticleUpdateRes struct{}

type HelpArticleDeleteReq struct {
	g.Meta `path:"/help-articles/{id}" method:"delete" mime:"json" tags:"管理后台-帮助中心" summary:"删除帮助文章"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type HelpArticleDeleteRes struct{}

type HelpArticleListReq struct {
	g.Meta     `path:"/help-articles" method:"get" mime:"json" tags:"管理后台-帮助中心" summary:"帮助文章列表"`
	CategoryId int64  `json:"category_id" in:"query" dc:"按分类过滤"`
	Status     string `json:"status" in:"query" dc:"按状态过滤"`
	Page       int    `json:"page" in:"query" d:"1"`
	PageSize   int    `json:"page_size" in:"query" d:"20"`
}

type HelpArticleItem struct {
	Id          int64       `json:"id"`
	CategoryId  int64       `json:"category_id"`
	Title       string      `json:"title"`
	Slug        string      `json:"slug"`
	Summary     string      `json:"summary"`
	Status      string      `json:"status"`
	AuthorId    int64       `json:"author_id"`
	ViewCount   int         `json:"view_count"`
	SortOrder   int         `json:"sort_order"`
	Keywords    []string    `json:"keywords"`
	PublishedAt *gtime.Time `json:"published_at,omitempty"`
	CreatedAt   *gtime.Time `json:"created_at"`
	UpdatedAt   *gtime.Time `json:"updated_at"`
}

type HelpArticleListRes struct {
	List     []*HelpArticleItem `json:"list"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type HelpArticleGetReq struct {
	g.Meta `path:"/help-articles/{id}" method:"get" mime:"json" tags:"管理后台-帮助中心" summary:"帮助文章详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type HelpArticleGetRes struct {
	Id          int64       `json:"id"`
	CategoryId  int64       `json:"category_id"`
	Title       string      `json:"title"`
	Slug        string      `json:"slug"`
	Content     string      `json:"content"`
	Summary     string      `json:"summary"`
	Status      string      `json:"status"`
	AuthorId    int64       `json:"author_id"`
	ViewCount   int         `json:"view_count"`
	SortOrder   int         `json:"sort_order"`
	Keywords    []string    `json:"keywords"`
	PublishedAt *gtime.Time `json:"published_at,omitempty"`
	CreatedAt   *gtime.Time `json:"created_at"`
	UpdatedAt   *gtime.Time `json:"updated_at"`
}
