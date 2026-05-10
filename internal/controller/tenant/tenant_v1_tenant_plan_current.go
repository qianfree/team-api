package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPlanCurrent(ctx context.Context, req *v1.TenantPlanCurrentReq) (res *v1.TenantPlanCurrentRes, err error) {
	return service.Tenant().PlanCurrent(ctx, req)
}
