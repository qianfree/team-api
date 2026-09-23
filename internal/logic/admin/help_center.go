package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ============================================================
// 帮助分类 CRUD
// ============================================================

// CreateHelpCategory 创建帮助分类
func (s *sAdmin) CreateHelpCategory(ctx context.Context, req *v1.HelpCategoryCreateReq) (*v1.HelpCategoryCreateRes, error) {
	// 检查 slug 唯一性
	count, err := dao.SptCategories.Ctx(ctx).Where("slug", req.Slug).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeHelpCategorySlugExists, consts.MsgHelpCategorySlugExists)
	}

	// 父分类校验：必须存在、必须是顶级分类（仅支持两级）
	if req.ParentId > 0 {
		if err = s.validateHelpCategoryParent(ctx, 0, req.ParentId); err != nil {
			return nil, err
		}
	}

	isVisible := true
	if req.IsVisible != nil {
		isVisible = *req.IsVisible
	}

	result, err := dao.SptCategories.Ctx(ctx).Data(do.SptCategories{
		ParentId:    req.ParentId,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		SortOrder:   req.SortOrder,
		Icon:        req.Icon,
		IsVisible:   isVisible,
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.HelpCategoryCreateRes{Id: id}, nil
}

// UpdateHelpCategory 更新帮助分类
func (s *sAdmin) UpdateHelpCategory(ctx context.Context, req *v1.HelpCategoryUpdateReq) (*v1.HelpCategoryUpdateRes, error) {
	var cat *struct {
		Id int64 `json:"id"`
	}
	err := dao.SptCategories.Ctx(ctx).Where("id", req.Id).Scan(&cat)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, common.NewBusinessError(consts.CodeHelpCategoryNotFound, consts.MsgHelpCategoryNotFound)
	}

	// 检查 slug 唯一性（排除自身）
	count, err := dao.SptCategories.Ctx(ctx).Where("slug", req.Slug).WhereNot("id", req.Id).Count()
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

	// 父分类校验：必须存在、必须是顶级分类（仅支持两级），且祖先链中不能出现自己（防环导致分类互相锁定无法删除）
	if req.ParentId > 0 {
		if err = s.validateHelpCategoryParent(ctx, req.Id, req.ParentId); err != nil {
			return nil, err
		}
	}

	updateData := do.SptCategories{
		ParentId:    req.ParentId,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		SortOrder:   req.SortOrder,
		Icon:        req.Icon,
	}
	if req.IsVisible != nil {
		updateData.IsVisible = *req.IsVisible
	}

	_, err = dao.SptCategories.Ctx(ctx).
		Where("id", req.Id).
		Data(updateData).
		Update()
	if err != nil {
		return nil, err
	}

	return &v1.HelpCategoryUpdateRes{}, nil
}

// validateHelpCategoryParent 校验父分类：必须存在、必须是顶级分类（仅支持两级结构），
// 且父分类的祖先链中不能出现 selfId（否则会形成 A→B→A 的环，环上分类因互为父子被删除校验拦截，永远无法删除）
func (s *sAdmin) validateHelpCategoryParent(ctx context.Context, selfId, parentId int64) error {
	var parent *struct {
		Id       int64 `json:"id"`
		ParentId int64 `json:"parent_id"`
	}
	err := dao.SptCategories.Ctx(ctx).Where("id", parentId).Scan(&parent)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return err
	}
	if parent == nil {
		return common.NewBusinessError(consts.CodeHelpCategoryNotFound, consts.MsgHelpCategoryNotFound)
	}
	if parent.ParentId != 0 {
		return common.NewBusinessError(consts.CodeBadRequest, "仅支持两级分类，父分类必须是顶级分类")
	}

	// 沿祖先链向上检查是否会出现自己（数据异常时用深度上限兜底）
	current := parent.ParentId
	for i := 0; i < 50 && current != 0; i++ {
		if current == selfId {
			return common.NewBusinessError(consts.CodeBadRequest, "不能将分类挂到自身或其子分类下")
		}
		var ancestor *struct {
			ParentId int64 `json:"parent_id"`
		}
		err = dao.SptCategories.Ctx(ctx).Where("id", current).Scan(&ancestor)
		if err = common.IgnoreScanNoRows(err); err != nil {
			return err
		}
		if ancestor == nil {
			return nil
		}
		current = ancestor.ParentId
	}
	return nil
}

// DeleteHelpCategory 删除帮助分类
func (s *sAdmin) DeleteHelpCategory(ctx context.Context, req *v1.HelpCategoryDeleteReq) (*v1.HelpCategoryDeleteRes, error) {
	var cat *struct {
		Id int64 `json:"id"`
	}
	err := dao.SptCategories.Ctx(ctx).Where("id", req.Id).Scan(&cat)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, common.NewBusinessError(consts.CodeHelpCategoryNotFound, consts.MsgHelpCategoryNotFound)
	}

	// 检查是否有子分类
	childCount, err := dao.SptCategories.Ctx(ctx).Where("parent_id", req.Id).Count()
	if err != nil {
		return nil, err
	}
	if childCount > 0 {
		return nil, common.NewBusinessError(consts.CodeBadRequest, "该分类下有子分类，请先删除子分类")
	}

	// 检查分类下是否有文章
	articleCount, err := dao.SptArticles.Ctx(ctx).Where("category_id", req.Id).Count()
	if err != nil {
		return nil, err
	}
	if articleCount > 0 {
		return nil, common.NewBusinessError(consts.CodeBadRequest, "该分类下有文章，请先删除或移动文章")
	}

	_, err = dao.SptCategories.Ctx(ctx).Where("id", req.Id).Delete()
	if err != nil {
		return nil, err
	}

	return &v1.HelpCategoryDeleteRes{}, nil
}

// ListHelpCategories 帮助分类列表（管理后台）
func (s *sAdmin) ListHelpCategories(ctx context.Context, req *v1.HelpCategoryListReq) (*v1.HelpCategoryListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.SptCategories.Ctx(ctx)
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
	var cat *struct {
		Id int64 `json:"id"`
	}
	err := dao.SptCategories.Ctx(ctx).Where("id", req.CategoryId).Scan(&cat)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, common.NewBusinessError(consts.CodeHelpCategoryNotFound, consts.MsgHelpCategoryNotFound)
	}

	// 检查 slug 唯一性
	count, err := dao.SptArticles.Ctx(ctx).Where("slug", req.Slug).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeHelpArticleSlugExists, consts.MsgHelpArticleSlugExists)
	}

	data := do.SptArticles{
		CategoryId: req.CategoryId,
		Title:      req.Title,
		Slug:       req.Slug,
		Content:    req.Content,
		Summary:    req.Summary,
		Status:     req.Status,
		AuthorId:   common.GetCtxUserID(ctx),
		SortOrder:  req.SortOrder,
		Keywords:   req.Keywords,
	}
	if req.Status == "published" {
		data.PublishedAt = gtime.Now()
	}

	result, err := dao.SptArticles.Ctx(ctx).Data(data).Insert()
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
	var article *struct {
		Id          int64       `json:"id"`
		CategoryId  int64       `json:"category_id"`
		Status      string      `json:"status"`
		PublishedAt *gtime.Time `json:"published_at"`
	}
	err := dao.SptArticles.Ctx(ctx).Where("id", req.Id).Scan(&article)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if article == nil {
		return nil, common.NewBusinessError(consts.CodeHelpArticleNotFound, consts.MsgHelpArticleNotFound)
	}

	// 检查 slug 唯一性（排除自身）
	count, err := dao.SptArticles.Ctx(ctx).Where("slug", req.Slug).WhereNot("id", req.Id).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeHelpArticleSlugExists, consts.MsgHelpArticleSlugExists)
	}

	// 检查分类是否存在
	var cat *struct {
		Id int64 `json:"id"`
	}
	err = dao.SptCategories.Ctx(ctx).Where("id", req.CategoryId).Scan(&cat)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, common.NewNotFoundError("分类")
	}

	data := do.SptArticles{
		CategoryId: req.CategoryId,
		Title:      req.Title,
		Slug:       req.Slug,
		Summary:    req.Summary,
		Status:     req.Status,
		SortOrder:  req.SortOrder,
		Keywords:   req.Keywords,
	}
	if req.Content != "" {
		data.Content = req.Content
	}

	// 从草稿发布时设置发布时间（仅首次发布固定；下架再上架不重置，保持原始发布时间与排序位置）
	if req.Status == "published" && article.Status != "published" && article.PublishedAt == nil {
		data.PublishedAt = gtime.Now()
	}

	_, err = dao.SptArticles.Ctx(ctx).
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

	// 状态变更（草稿↔已发布）也需要刷新计数
	if article.Status != req.Status {
		s.refreshCategoryArticleCount(ctx, req.CategoryId)
	}

	return &v1.HelpArticleUpdateRes{}, nil
}

// DeleteHelpArticle 删除帮助文章
func (s *sAdmin) DeleteHelpArticle(ctx context.Context, req *v1.HelpArticleDeleteReq) (*v1.HelpArticleDeleteRes, error) {
	var article *struct {
		Id         int64 `json:"id"`
		CategoryId int64 `json:"category_id"`
	}
	err := dao.SptArticles.Ctx(ctx).Where("id", req.Id).Scan(&article)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if article == nil {
		return nil, common.NewBusinessError(consts.CodeHelpArticleNotFound, consts.MsgHelpArticleNotFound)
	}

	_, err = dao.SptArticles.Ctx(ctx).Where("id", req.Id).Delete()
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

	query := dao.SptArticles.Ctx(ctx)
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
	var article *v1.HelpArticleGetRes
	err := dao.SptArticles.Ctx(ctx).Where("id", req.Id).Scan(&article)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if article == nil {
		return nil, common.NewBusinessError(consts.CodeHelpArticleNotFound, consts.MsgHelpArticleNotFound)
	}

	return article, nil
}

// refreshCategoryArticleCount 刷新分类的文章计数
func (s *sAdmin) refreshCategoryArticleCount(ctx context.Context, categoryId int64) {
	count, err := dao.SptArticles.Ctx(ctx).
		Where("category_id", categoryId).
		Where("status", "published").
		Where("published_at IS NOT NULL").
		Where("published_at <=", gtime.Now()).
		Count()
	if err != nil {
		g.Log().Warningf(ctx, "refreshCategoryArticleCount count failed for category %d: %v", categoryId, err)
		return
	}
	_, err = dao.SptCategories.Ctx(ctx).
		Where("id", categoryId).
		Data(do.SptCategories{ArticleCount: count}).
		Update()
	if err != nil {
		g.Log().Warningf(ctx, "refreshCategoryArticleCount update failed for category %d: %v", categoryId, err)
	}
}
