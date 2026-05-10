package common

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ============================================================
// 公开帮助中心逻辑
// ============================================================

// HelpPublicCategoryItem 公开分类树节点
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

// ListPublicCategories 返回可见分类的树结构
func ListPublicCategories(ctx context.Context) ([]*HelpPublicCategoryItem, error) {
	type categoryRow struct {
		Id           int64  `json:"id" orm:"id"`
		ParentId     int64  `json:"parent_id" orm:"parent_id"`
		Name         string `json:"name" orm:"name"`
		Slug         string `json:"slug" orm:"slug"`
		Description  string `json:"description" orm:"description"`
		Icon         string `json:"icon" orm:"icon"`
		IsVisible    bool   `json:"is_visible" orm:"is_visible"`
		ArticleCount int    `json:"article_count" orm:"article_count"`
	}

	var rows []categoryRow
	err := g.DB().Model("spt_categories").Ctx(ctx).
		Where("is_visible", true).
		OrderAsc("sort_order").
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	// 构建所有节点
	nodeMap := make(map[int64]*HelpPublicCategoryItem)
	for _, r := range rows {
		nodeMap[r.Id] = &HelpPublicCategoryItem{
			Id:           r.Id,
			ParentId:     r.ParentId,
			Name:         r.Name,
			Slug:         r.Slug,
			Description:  r.Description,
			Icon:         r.Icon,
			ArticleCount: r.ArticleCount,
			Children:     make([]*HelpPublicCategoryItem, 0),
		}
	}

	// 组装树结构
	var roots []*HelpPublicCategoryItem
	for _, node := range nodeMap {
		if node.ParentId == 0 {
			roots = append(roots, node)
		} else if parent, ok := nodeMap[node.ParentId]; ok {
			parent.Children = append(parent.Children, node)
		}
	}

	return roots, nil
}

// HelpPublicArticleItem 公开文章摘要
type HelpPublicArticleItem struct {
	Id          int64       `json:"id"`
	CategoryId  int64       `json:"category_id"`
	Title       string      `json:"title"`
	Slug        string      `json:"slug"`
	Summary     string      `json:"summary"`
	ViewCount   int         `json:"view_count"`
	PublishedAt *gtime.Time `json:"published_at"`
}

// ListPublicArticles 按分类 slug 获取已发布文章列表
func ListPublicArticles(ctx context.Context, categorySlug string, page, pageSize int) ([]*HelpPublicArticleItem, int, int, int, error) {
	page, pageSize = NormalizePagination(page, pageSize)

	// 查找分类
	var cat struct {
		Id int64 `json:"id" orm:"id"`
	}
	err := g.DB().Model("spt_categories").Ctx(ctx).
		Where("slug", categorySlug).
		Where("is_visible", true).
		Scan(&cat)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if cat.Id == 0 {
		return nil, 0, 0, 0, NewBusinessError(10073, "帮助分类不存在")
	}

	var total int
	rows := make([]*HelpPublicArticleItem, 0)
	err = g.DB().Model("spt_articles").Ctx(ctx).
		Where("category_id", cat.Id).
		Where("status", "published").
		Where("published_at IS NOT NULL").
		Where("published_at <=", gtime.Now()).
		OrderDesc("published_at").
		OrderAsc("sort_order").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	return rows, total, page, pageSize, nil
}

// HelpPublicArticleDetail 公开文章详情
type HelpPublicArticleDetail struct {
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

// GetPublicArticle 按 slug 获取文章详情，并增加浏览计数
func GetPublicArticle(ctx context.Context, slug string) (*HelpPublicArticleDetail, error) {
	type articleRow struct {
		Id          int64       `json:"id" orm:"id"`
		CategoryId  int64       `json:"category_id" orm:"category_id"`
		Title       string      `json:"title" orm:"title"`
		Slug        string      `json:"slug" orm:"slug"`
		Content     string      `json:"content" orm:"content"`
		Summary     string      `json:"summary" orm:"summary"`
		Status      string      `json:"status" orm:"status"`
		ViewCount   int         `json:"view_count" orm:"view_count"`
		Keywords    []string    `json:"keywords" orm:"keywords"`
		PublishedAt *gtime.Time `json:"published_at" orm:"published_at"`
	}

	var row articleRow
	err := g.DB().Model("spt_articles").Ctx(ctx).
		Where("slug", slug).
		Scan(&row)
	if err != nil {
		return nil, err
	}
	if row.Id == 0 {
		return nil, NewBusinessError(10075, "帮助文章不存在")
	}
	if row.Status != "published" || row.PublishedAt == nil || row.PublishedAt.Timestamp() > gtime.Now().Timestamp() {
		return nil, NewBusinessError(10075, "帮助文章不存在")
	}

	// 增加浏览计数
	g.DB().Model("spt_articles").Ctx(ctx).
		Where("id", row.Id).
		Increment("view_count", 1)

	return &HelpPublicArticleDetail{
		Id:          row.Id,
		CategoryId:  row.CategoryId,
		Title:       row.Title,
		Slug:        row.Slug,
		Content:     row.Content,
		Summary:     row.Summary,
		ViewCount:   row.ViewCount + 1,
		Keywords:    row.Keywords,
		PublishedAt: row.PublishedAt,
	}, nil
}

// SearchPublicArticles 全文搜索已发布文章
func SearchPublicArticles(ctx context.Context, query string, page, pageSize int) ([]*HelpPublicArticleItem, int, int, int, error) {
	page, pageSize = NormalizePagination(page, pageSize)

	var total int
	rows := make([]*HelpPublicArticleItem, 0)
	err := g.DB().Model("spt_articles").Ctx(ctx).
		Where("status", "published").
		Where("published_at IS NOT NULL").
		Where("published_at <=", gtime.Now()).
		Where("to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(content,'')) @@ plainto_tsquery('simple', ?)", query).
		OrderDesc("published_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	return rows, total, page, pageSize, nil
}
