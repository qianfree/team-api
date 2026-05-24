package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPlanOrderCreate(ctx context.Context, req *v1.TenantPlanOrderCreateReq) (res *v1.TenantPlanOrderCreateRes, err error) {
	return service.Tenant().PlanOrderCreate(ctx, req)
}
