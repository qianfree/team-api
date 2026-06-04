package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpCategoryCreate(ctx context.Context, req *v1.HelpCategoryCreateReq) (res *v1.HelpCategoryCreateRes, err error) {
	return service.Admin().CreateHelpCategory(ctx, req)
}
func (c *ControllerV1) HelpCategoryUpdate(ctx context.Context, req *v1.HelpCategoryUpdateReq) (res *v1.HelpCategoryUpdateRes, err error) {
	return service.Admin().UpdateHelpCategory(ctx, req)
}
func (c *ControllerV1) HelpCategoryDelete(ctx context.Context, req *v1.HelpCategoryDeleteReq) (res *v1.HelpCategoryDeleteRes, err error) {
	return service.Admin().DeleteHelpCategory(ctx, req)
}
func (c *ControllerV1) HelpCategoryList(ctx context.Context, req *v1.HelpCategoryListReq) (res *v1.HelpCategoryListRes, err error) {
	return service.Admin().ListHelpCategories(ctx, req)
}
func (c *ControllerV1) HelpArticleCreate(ctx context.Context, req *v1.HelpArticleCreateReq) (res *v1.HelpArticleCreateRes, err error) {
	return service.Admin().CreateHelpArticle(ctx, req)
}
func (c *ControllerV1) HelpArticleUpdate(ctx context.Context, req *v1.HelpArticleUpdateReq) (res *v1.HelpArticleUpdateRes, err error) {
	return service.Admin().UpdateHelpArticle(ctx, req)
}
func (c *ControllerV1) HelpArticleDelete(ctx context.Context, req *v1.HelpArticleDeleteReq) (res *v1.HelpArticleDeleteRes, err error) {
	return service.Admin().DeleteHelpArticle(ctx, req)
}
func (c *ControllerV1) HelpArticleList(ctx context.Context, req *v1.HelpArticleListReq) (res *v1.HelpArticleListRes, err error) {
	return service.Admin().ListHelpArticles(ctx, req)
}
func (c *ControllerV1) HelpArticleGet(ctx context.Context, req *v1.HelpArticleGetReq) (res *v1.HelpArticleGetRes, err error) {
	return service.Admin().GetHelpArticle(ctx, req)
}
