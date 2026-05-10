package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpPublicArticleGet(ctx context.Context, req *v1.HelpPublicArticleGetReq) (res *v1.HelpPublicArticleGetRes, err error) {
	return service.Tenant().GetHelpPublicArticle(ctx, req)
}
