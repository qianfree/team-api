package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpPublicCategoryList(ctx context.Context, req *v1.HelpPublicCategoryListReq) (res *v1.HelpPublicCategoryListRes, err error) {
	return service.Tenant().ListHelpPublicCategories(ctx, req)
}
