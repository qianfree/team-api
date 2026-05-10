package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/logic/common"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ============================================================
// 帮助分类 CRUD
// ============================================================

// CreateHelpCategory 创建帮助分类
func (s *sAdmin) CreateHelpCategory(ctx context.Context, req *v1.HelpCategoryCreateReq) (*v1.HelpCategoryCreateRes, error) {
	// 检查 slug 唯一性
	count, err := g.DB().Model("spt_categories").Ctx(ctx).Where("slug", req.Slug).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeHelpCategorySlugExists, consts.MsgHelpCategorySlugExists)
	}

	isVisible := true
	if req.IsVisible != nil {
		isVisible = *req.IsVisible
	}

	result, err := g.DB().Model("spt_categories").Ctx(ctx).Data(g.Map{
		"parent_id":   req.ParentId,
		"name":        req.Name,
		"slug":        req.Slug,
		"description": req.Description,
		"sort_order":  req.SortOrder,
		"icon":        req.Icon,
		"is_visible":  isVisible,
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.HelpCategoryCreateRes{Id: id}, nil
}

// UpdateHelpCategory 更新帮助分类
func (s *sAdmin) UpdateHelpCategory(ctx context.Context, req *v1.HelpCategoryUpdateReq) (*v1.HelpCategoryUpdateRes, error) {
	var cat struct {
		Id int64 `json:"id"`
	}
	err := g.DB().Model("spt_categories").Ctx(ctx).Where("id", req.Id).Scan(&cat)
	if err != nil {
		return nil, err
	}
	if cat.Id == 0 {
		return nil, common.NewBusinessError(consts.CodeHelpCategoryNotFound, consts.MsgHelpCategoryNotFound)
	}

	// 检查 slug 唯一性（排除自身）
	count, err := g.DB().Model("spt_categories").Ctx(ctx).Where("slug", req.Slug).WhereNot("id", req.Id).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeHelpCategorySlugExists, consts.MsgHelpCategorySlugExists)
	}

	// 不允许将分类设置为自己的子分类
	if req.ParentId > 0 && req.ParentId == req.Id {
		return nil, common.NewBusinessError(consts.CodeBadRequest, "不能将分类设置为自己的子分类")
	}

	updateData := g.Map{
		"parent_id":   req.ParentId,
		"name":        req.Name,
		"slug":        req.Slug,
		"description": req.Description,
		"sort_order":  req.SortOrder,
		"icon":        req.Icon,
	}
	if req.IsVisible != nil {
		updateData["is_visible"] = *req.IsVisible
	}

	_, err = g.DB().Model("spt_categories").Ctx(ctx).
		Where("id", req.Id).
		Data(updateData).
		Update()
	if err != nil {
		return nil, err
	}

	return &v1.HelpCategoryUpdateRes{}, nil
}

// DeleteHelpCategory 删除帮助分类
func (s *sAdmin) DeleteHelpCategory(ctx context.Context, req *v1.HelpCategoryDeleteReq) (*v1.HelpCategoryDeleteRes, error) {
	var cat struct {
		Id int64 `json:"id"`
	}
	err := g.DB().Model("spt_categories").Ctx(ctx).Where("id", req.Id).Scan(&cat)
	if err != nil {
		return nil, err
	}
	if cat.Id == 0 {
		return nil, common.NewBusinessError(consts.CodeHelpCategoryNotFound, consts.MsgHelpCategoryNotFound)
	}

	// 检查是否有子分类
	childCount, err := g.DB().Model("spt_categories").Ctx(ctx).Where("parent_id", req.Id).Count()
	if err != nil {
		return nil, err
	}
	if childCount > 0 {
		return nil, common.NewBusinessError(consts.CodeBadRequest, "该分类下有子分类，请先删除子分类")
	}

	// 检查分类下是否有文章
	articleCount, err := g.DB().Model("spt_articles").Ctx(ctx).Where("category_id", req.Id).Count()
	if err != nil {
		return nil, err
	}
	if articleCount > 0 {
		return nil, common.NewBusinessError(consts.CodeBadRequest, "该分类下有文章，请先删除或移动文章")
	}

	_, err = g.DB().Model("spt_categories").Ctx(ctx).Where("id", req.Id).Delete()
	if err != nil {
		return nil, err
	}

	return &v1.HelpCategoryDeleteRes{}, nil
}

// ListHelpCategories 帮助分类列表（管理后台）
func (s *sAdmin) ListHelpCategories(ctx context.Context, req *v1.HelpCategoryListReq) (*v1.HelpCategoryListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := g.DB().Model("spt_categories").Ctx(ctx)
	if req.ParentId >= 0 {
		query = query.Where("parent_id", req.ParentId)
	}

	var total int
	rows := make([]*v1.HelpCategoryItem, 0)
	err := query.OrderAsc("sort_order").OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.HelpCategoryListRes{
		List:     rows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ============================================================
// 帮助文章 CRUD
// ============================================================

// CreateHelpArticle 创建帮助文章
func (s *sAdmin) CreateHelpArticle(ctx context.Context, req *v1.HelpArticleCreateReq) (*v1.HelpArticleCreateRes, error) {
	// 检查分类是否存在
	var cat struct {
		Id int64 `json:"id"`
	}
	err := g.DB().Model("spt_categories").Ctx(ctx).Where("id", req.CategoryId).Scan(&cat)
	if err != nil {
		return nil, err
	}
	if cat.Id == 0 {
		return nil, common.NewBusinessError(consts.CodeHelpCategoryNotFound, consts.MsgHelpCategoryNotFound)
	}

	// 检查 slug 唯一性
	count, err := g.DB().Model("spt_articles").Ctx(ctx).Where("slug", req.Slug).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeHelpArticleSlugExists, consts.MsgHelpArticleSlugExists)
	}

	data := g.Map{
		"category_id": req.CategoryId,
		"title":       req.Title,
		"slug":        req.Slug,
		"content":     req.Content,
		"summary":     req.Summary,
		"status":      req.Status,
		"author_id":   getCtxUserID(ctx),
		"sort_order":  req.SortOrder,
		"keywords":    req.Keywords,
	}
	if req.Status == "published" {
		data["published_at"] = gtime.Now()
	}

	result, err := g.DB().Model("spt_articles").Ctx(ctx).Data(data).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()

	// 更新分类的文章计数
	s.refreshCategoryArticleCount(ctx, req.CategoryId)

	return &v1.HelpArticleCreateRes{Id: id}, nil
}

// UpdateHelpArticle 更新帮助文章
func (s *sAdmin) UpdateHelpArticle(ctx context.Context, req *v1.HelpArticleUpdateReq) (*v1.HelpArticleUpdateRes, error) {
	var article struct {
		Id         int64  `json:"id"`
		CategoryId int64  `json:"category_id"`
		Status     string `json:"status"`
	}
	err := g.DB().Model("spt_articles").Ctx(ctx).Where("id", req.Id).Scan(&article)
	if err != nil {
		return nil, err
	}
	if article.Id == 0 {
		return nil, common.NewBusinessError(consts.CodeHelpArticleNotFound, consts.MsgHelpArticleNotFound)
	}

	// 检查 slug 唯一性（排除自身）
	count, err := g.DB().Model("spt_articles").Ctx(ctx).Where("slug", req.Slug).WhereNot("id", req.Id).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeHelpArticleSlugExists, consts.MsgHelpArticleSlugExists)
	}

	// 检查分类是否存在
	var cat struct {
		Id int64 `json:"id"`
	}
	err = g.DB().Model("spt_categories").Ctx(ctx).Where("id", req.CategoryId).Scan(&cat)
	if err != nil {
		return nil, err
	}
	if cat.Id == 0 {
		return nil, common.NewBusinessError(consts.CodeHelpCategoryNotFound, consts.MsgHelpCategoryNotFound)
	}

	data := g.Map{
		"category_id": req.CategoryId,
		"title":       req.Title,
		"slug":        req.Slug,
		"summary":     req.Summary,
		"status":      req.Status,
		"sort_order":  req.SortOrder,
		"keywords":    req.Keywords,
	}
	if req.Content != "" {
		data["content"] = req.Content
	}

	// 从草稿发布时设置发布时间
	if req.Status == "published" && article.Status != "published" {
		data["published_at"] = gtime.Now()
	}

	_, err = g.DB().Model("spt_articles").Ctx(ctx).
		Where("id", req.Id).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}

	// 更新旧分类和新分类的文章计数
	if article.CategoryId != req.CategoryId {
		s.refreshCategoryArticleCount(ctx, article.CategoryId)
		s.refreshCategoryArticleCount(ctx, req.CategoryId)
	}

	return &v1.HelpArticleUpdateRes{}, nil
}

// DeleteHelpArticle 删除帮助文章
func (s *sAdmin) DeleteHelpArticle(ctx context.Context, req *v1.HelpArticleDeleteReq) (*v1.HelpArticleDeleteRes, error) {
	var article struct {
		Id         int64 `json:"id"`
		CategoryId int64 `json:"category_id"`
	}
	err := g.DB().Model("spt_articles").Ctx(ctx).Where("id", req.Id).Scan(&article)
	if err != nil {
		return nil, err
	}
	if article.Id == 0 {
		return nil, common.NewBusinessError(consts.CodeHelpArticleNotFound, consts.MsgHelpArticleNotFound)
	}

	_, err = g.DB().Model("spt_articles").Ctx(ctx).Where("id", req.Id).Delete()
	if err != nil {
		return nil, err
	}

	// 更新分类的文章计数
	s.refreshCategoryArticleCount(ctx, article.CategoryId)

	return &v1.HelpArticleDeleteRes{}, nil
}

// ListHelpArticles 帮助文章列表（管理后台）
func (s *sAdmin) ListHelpArticles(ctx context.Context, req *v1.HelpArticleListReq) (*v1.HelpArticleListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := g.DB().Model("spt_articles").Ctx(ctx)
	if req.CategoryId > 0 {
		query = query.Where("category_id", req.CategoryId)
	}
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}

	var total int
	rows := make([]*v1.HelpArticleItem, 0)
	err := query.OrderAsc("sort_order").OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.HelpArticleListRes{
		List:     rows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetHelpArticle 帮助文章详情（管理后台）
func (s *sAdmin) GetHelpArticle(ctx context.Context, req *v1.HelpArticleGetReq) (*v1.HelpArticleGetRes, error) {
	var article v1.HelpArticleGetRes
	err := g.DB().Model("spt_articles").Ctx(ctx).Where("id", req.Id).Scan(&article)
	if err != nil {
		return nil, err
	}
	if article.Id == 0 {
		return nil, common.NewBusinessError(consts.CodeHelpArticleNotFound, consts.MsgHelpArticleNotFound)
	}

	return &article, nil
}

// refreshCategoryArticleCount 刷新分类的文章计数
func (s *sAdmin) refreshCategoryArticleCount(ctx context.Context, categoryId int64) {
	count, err := g.DB().Model("spt_articles").Ctx(ctx).
		Where("category_id", categoryId).
		Where("status", "published").
		Count()
	if err != nil {
		return
	}
	g.DB().Model("spt_categories").Ctx(ctx).
		Where("id", categoryId).
		Data(g.Map{"article_count": count}).
		Update()
}
