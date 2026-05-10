package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 帮助中心（公开接口） ===

type HelpPublicCategoryListReq struct {
	g.Meta `path:"/help/categories" method:"get" mime:"json" tags:"公开-帮助中心" summary:"帮助分类列表（树结构）" group:"public" middleware:"-"`
}

type HelpPublicCategoryItem struct {
	Id           int64                     `json:"id"`
	ParentId     int64                     `json:"parent_id"`
	Name         string                    `json:"name"`
	Slug         string                    `json:"slug"`
	Description  string                    `json:"description"`
	Icon         string                    `json:"icon"`
	ArticleCount int                       `json:"article_count"`
	Children     []*HelpPublicCategoryItem `json:"children"`
}

type HelpPublicCategoryListRes struct {
	List []*HelpPublicCategoryItem `json:"list"`
}

type HelpPublicArticleListReq struct {
	g.Meta       `path:"/help/categories/{categorySlug}/articles" method:"get" mime:"json" tags:"公开-帮助中心" summary:"分类下的文章列表" group:"public" middleware:"-"`
	CategorySlug string `json:"category_slug" in:"path" v:"required" dc:"分类slug"`
	Page         int    `json:"page" in:"query" d:"1"`
	PageSize     int    `json:"page_size" in:"query" d:"20"`
}

type HelpPublicArticleItem struct {
	Id          int64       `json:"id"`
	CategoryId  int64       `json:"category_id"`
	Title       string      `json:"title"`
	Slug        string      `json:"slug"`
	Summary     string      `json:"summary"`
	ViewCount   int         `json:"view_count"`
	PublishedAt *gtime.Time `json:"published_at"`
}

type HelpPublicArticleListRes struct {
	List     []*HelpPublicArticleItem `json:"list"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type HelpPublicArticleGetReq struct {
	g.Meta `path:"/help/articles/{slug}" method:"get" mime:"json" tags:"公开-帮助中心" summary:"文章详情" group:"public" middleware:"-"`
	Slug   string `json:"slug" in:"path" v:"required" dc:"文章slug"`
}

type HelpPublicArticleGetRes struct {
	Id          int64       `json:"id"`
	CategoryId  int64       `json:"category_id"`
	Title       string      `json:"title"`
	Slug        string      `json:"slug"`
	Content     string      `json:"content"`
	Summary     string      `json:"summary"`
	ViewCount   int         `json:"view_count"`
	Keywords    []string    `json:"keywords"`
	PublishedAt *gtime.Time `json:"published_at"`
}

type HelpPublicSearchReq struct {
	g.Meta   `path:"/help/search" method:"get" mime:"json" tags:"公开-帮助中心" summary:"搜索文章" group:"public" middleware:"-"`
	Query    string `json:"q" in:"query" v:"required|length:1,200" dc:"搜索关键词"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
}

type HelpPublicSearchRes struct {
	List     []*HelpPublicArticleItem `json:"list"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}
