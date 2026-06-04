package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpPublicCategoryList(ctx context.Context, req *v1.HelpPublicCategoryListReq) (res *v1.HelpPublicCategoryListRes, err error) {
	return service.Tenant().ListHelpPublicCategories(ctx, req)
}
func (c *ControllerV1) HelpPublicArticleList(ctx context.Context, req *v1.HelpPublicArticleListReq) (res *v1.HelpPublicArticleListRes, err error) {
	return service.Tenant().ListHelpPublicArticles(ctx, req)
}
func (c *ControllerV1) HelpPublicArticleGet(ctx context.Context, req *v1.HelpPublicArticleGetReq) (res *v1.HelpPublicArticleGetRes, err error) {
	return service.Tenant().GetHelpPublicArticle(ctx, req)
}
func (c *ControllerV1) HelpPublicSearch(ctx context.Context, req *v1.HelpPublicSearchReq) (res *v1.HelpPublicSearchRes, err error) {
	return service.Tenant().SearchHelpPublicArticles(ctx, req)
}
