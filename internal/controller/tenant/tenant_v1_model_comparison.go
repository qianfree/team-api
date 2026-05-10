package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelComparison(ctx context.Context, req *v1.ModelComparisonReq) (res *v1.ModelComparisonRes, err error) {
	return service.Tenant().ModelComparison(ctx, req)
}
