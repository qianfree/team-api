package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpPublicSearch(ctx context.Context, req *v1.HelpPublicSearchReq) (res *v1.HelpPublicSearchRes, err error) {
	return service.Tenant().SearchHelpPublicArticles(ctx, req)
}
