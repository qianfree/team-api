package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRedemptionUsages(ctx context.Context, req *v1.TenantRedemptionUsagesReq) (res *v1.TenantRedemptionUsagesRes, err error) {
	return service.Tenant().ListRedemptionUsages(ctx, req)
}
