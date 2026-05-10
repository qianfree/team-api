package tenant

import (
	"context"

	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/logic/common"
)

// ListHelpPublicCategories 公开帮助分类列表（树结构）
func (s *sTenant) ListHelpPublicCategories(ctx context.Context, _ *v1.HelpPublicCategoryListReq) (*v1.HelpPublicCategoryListRes, error) {
	items, err := common.ListPublicCategories(ctx)
	if err != nil {
		return nil, err
	}

	list := convertPublicCategoryTree(items)
	return &v1.HelpPublicCategoryListRes{List: list}, nil
}

// ListHelpPublicArticles 分类下的文章列表
func (s *sTenant) ListHelpPublicArticles(ctx context.Context, req *v1.HelpPublicArticleListReq) (*v1.HelpPublicArticleListRes, error) {
	items, total, page, pageSize, err := common.ListPublicArticles(ctx, req.CategorySlug, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.HelpPublicArticleItem, 0, len(items))
	for _, item := range items {
		list = append(list, &v1.HelpPublicArticleItem{
			Id:          item.Id,
			CategoryId:  item.CategoryId,
			Title:       item.Title,
			Slug:        item.Slug,
			Summary:     item.Summary,
			ViewCount:   item.ViewCount,
			PublishedAt: item.PublishedAt,
		})
	}

	return &v1.HelpPublicArticleListRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetHelpPublicArticle 文章详情
func (s *sTenant) GetHelpPublicArticle(ctx context.Context, req *v1.HelpPublicArticleGetReq) (*v1.HelpPublicArticleGetRes, error) {
	detail, err := common.GetPublicArticle(ctx, req.Slug)
	if err != nil {
		return nil, err
	}

	var publishedAt *gtime.Time
	if detail.PublishedAt != nil {
		publishedAt = detail.PublishedAt
	}

	return &v1.HelpPublicArticleGetRes{
		Id:          detail.Id,
		CategoryId:  detail.CategoryId,
		Title:       detail.Title,
		Slug:        detail.Slug,
		Content:     detail.Content,
		Summary:     detail.Summary,
		ViewCount:   detail.ViewCount,
		Keywords:    detail.Keywords,
		PublishedAt: publishedAt,
	}, nil
}

// SearchHelpPublicArticles 搜索文章
func (s *sTenant) SearchHelpPublicArticles(ctx context.Context, req *v1.HelpPublicSearchReq) (*v1.HelpPublicSearchRes, error) {
	items, total, page, pageSize, err := common.SearchPublicArticles(ctx, req.Query, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.HelpPublicArticleItem, 0, len(items))
	for _, item := range items {
		list = append(list, &v1.HelpPublicArticleItem{
			Id:          item.Id,
			CategoryId:  item.CategoryId,
			Title:       item.Title,
			Slug:        item.Slug,
			Summary:     item.Summary,
			ViewCount:   item.ViewCount,
			PublishedAt: item.PublishedAt,
		})
	}

	return &v1.HelpPublicSearchRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func convertPublicCategoryTree(items []*common.HelpPublicCategoryItem) []*v1.HelpPublicCategoryItem {
	if items == nil {
		return make([]*v1.HelpPublicCategoryItem, 0)
	}
	list := make([]*v1.HelpPublicCategoryItem, 0, len(items))
	for _, item := range items {
		list = append(list, &v1.HelpPublicCategoryItem{
			Id:           item.Id,
			ParentId:     item.ParentId,
			Name:         item.Name,
			Slug:         item.Slug,
			Description:  item.Description,
			Icon:         item.Icon,
			ArticleCount: item.ArticleCount,
			Children:     convertPublicCategoryTree(item.Children),
		})
	}
	return list
}
