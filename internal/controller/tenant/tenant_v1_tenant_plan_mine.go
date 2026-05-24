package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPlanMine(ctx context.Context, req *v1.TenantPlanMineReq) (res *v1.TenantPlanMineRes, err error) {
	return service.Tenant().PlanMine(ctx, req)
}
