package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
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
		Id          int64  `json:"id" orm:"id"`
		ParentId    int64  `json:"parent_id" orm:"parent_id"`
		Name        string `json:"name" orm:"name"`
		Slug        string `json:"slug" orm:"slug"`
		Description string `json:"description" orm:"description"`
		Icon        string `json:"icon" orm:"icon"`
		IsVisible   bool   `json:"is_visible" orm:"is_visible"`
	}

	var rows []categoryRow
	err := dao.SptCategories.Ctx(ctx).
		Where("is_visible", true).
		OrderAsc("sort_order").
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	// 实时统计每个分类的已发布文章数（不依赖冗余字段 article_count）
	type countRow struct {
		CategoryId   int64 `json:"category_id" orm:"category_id"`
		ArticleCount int   `json:"count" orm:"count"`
	}
	var countRows []countRow
	err = dao.SptArticles.Ctx(ctx).
		Where("status", "published").
		Where("published_at IS NOT NULL").
		Where("published_at <=", gtime.Now()).
		Fields("category_id, COUNT(*) as count").
		Group("category_id").
		Scan(&countRows)
	if err != nil {
		return nil, err
	}
	countMap := make(map[int64]int, len(countRows))
	for _, cr := range countRows {
		countMap[cr.CategoryId] = cr.ArticleCount
	}

	// 构建所有节点（rows 已按 sort_order 排序，同时维护有序切片保证树节点顺序稳定，
	// 不能遍历 map 组装——Go map 遍历顺序随机，会导致分类顺序每次请求都变）
	nodeMap := make(map[int64]*HelpPublicCategoryItem, len(rows))
	orderedNodes := make([]*HelpPublicCategoryItem, 0, len(rows))
	for _, r := range rows {
		node := &HelpPublicCategoryItem{
			Id:           r.Id,
			ParentId:     r.ParentId,
			Name:         r.Name,
			Slug:         r.Slug,
			Description:  r.Description,
			Icon:         r.Icon,
			ArticleCount: countMap[r.Id],
			Children:     make([]*HelpPublicCategoryItem, 0),
		}
		nodeMap[r.Id] = node
		orderedNodes = append(orderedNodes, node)
	}

	// 组装树结构（按 sort_order 顺序遍历，roots 与 children 均保持稳定排序）
	var roots []*HelpPublicCategoryItem
	for _, node := range orderedNodes {
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
	var cat *struct {
		Id int64 `json:"id" orm:"id"`
	}
	err := dao.SptCategories.Ctx(ctx).
		Where("slug", categorySlug).
		Where("is_visible", true).
		Scan(&cat)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if cat == nil {
		return nil, 0, 0, 0, NewBusinessError(consts.CodeHelpCategoryNotFound, consts.MsgHelpCategoryNotFound)
	}

	var total int
	rows := make([]*HelpPublicArticleItem, 0)
	// 排序与管理后台列表保持一致：sort_order 优先，其次发布时间倒序
	err = dao.SptArticles.Ctx(ctx).
		Where("category_id", cat.Id).
		Where("status", "published").
		Where("published_at IS NOT NULL").
		Where("published_at <=", gtime.Now()).
		OrderAsc("sort_order").
		OrderDesc("published_at").
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
	err := dao.SptArticles.Ctx(ctx).
		Where("slug", slug).
		Scan(&row)
	if err != nil {
		return nil, err
	}
	if row.Id == 0 {
		return nil, NewBusinessError(consts.CodeHelpArticleNotFound, consts.MsgHelpArticleNotFound)
	}
	if row.Status != "published" || row.PublishedAt == nil || row.PublishedAt.Timestamp() > gtime.Now().Timestamp() {
		return nil, NewBusinessError(consts.CodeHelpArticleNotFound, consts.MsgHelpArticleNotFound)
	}

	// 增加浏览计数（失败仅记日志，不影响详情返回）
	if _, err = dao.SptArticles.Ctx(ctx).
		Where("id", row.Id).
		Increment("view_count", 1); err != nil {
		g.Log().Warningf(ctx, "帮助文章浏览计数失败 id=%d: %v", row.Id, err)
	}

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

// SearchPublicArticles 搜索已发布文章
// 用 ILIKE 模糊匹配而非 to_tsvector 全文检索：PostgreSQL 'simple' 分词配置不做中文分词，
// 连续中文会被切成单个长 token，中文关键词几乎无法命中（帮助文档以中文内容为主）。
// 帮助文章量级通常为百级，ILIKE 顺序扫描性能足够。
func SearchPublicArticles(ctx context.Context, query string, page, pageSize int) ([]*HelpPublicArticleItem, int, int, int, error) {
	page, pageSize = NormalizePagination(page, pageSize)

	likePat := "%" + escapeLikePattern(query) + "%"
	var total int
	rows := make([]*HelpPublicArticleItem, 0)
	err := dao.SptArticles.Ctx(ctx).
		Where("status", "published").
		Where("published_at IS NOT NULL").
		Where("published_at <=", gtime.Now()).
		Where("(title ILIKE ? ESCAPE '\\' OR content ILIKE ? ESCAPE '\\')", likePat, likePat).
		OrderDesc("published_at").
		OrderAsc("sort_order").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	return rows, total, page, pageSize, nil
}

// escapeLikePattern 转义 ILIKE 模式中的通配符（配合 ESCAPE '\' 使用），防止用户输入 % _ \ 干扰匹配
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// CheckHelpCenterRateLimit 帮助中心公开接口 IP 限流（公开无认证端点，防刷浏览量/搜索滥用）
// 默认每 IP 每分钟 60 次，可通过配置 help_center_rate_limit_per_minute 调整；Redis 不可用时放行（帮助中心为公开只读内容）
func CheckHelpCenterRateLimit(ctx context.Context, ipAddress string) error {
	if ipAddress == "" {
		return nil
	}
	limit := Config().GetInt(ctx, "help_center_rate_limit_per_minute")
	if limit <= 0 {
		limit = 60
	}
	key := fmt.Sprintf("help:center:rl:%s:%d", ipAddress, time.Now().Unix()/60)
	count, err := incrementWithExpire(ctx, key, 120)
	if err != nil {
		g.Log().Warningf(ctx, "帮助中心限流检查失败: %v", err)
		return nil
	}
	if count > int64(limit) {
		return NewBusinessError(consts.CodeHelpRateLimitExceeded, consts.MsgHelpRateLimitExceeded)
	}
	return nil
}
