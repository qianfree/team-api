package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpPublicArticleList(ctx context.Context, req *v1.HelpPublicArticleListReq) (res *v1.HelpPublicArticleListRes, err error) {
	return service.Tenant().ListHelpPublicArticles(ctx, req)
}
