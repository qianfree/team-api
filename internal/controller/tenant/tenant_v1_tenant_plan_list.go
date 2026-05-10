package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPlanList(ctx context.Context, req *v1.TenantPlanListReq) (res *v1.TenantPlanListRes, err error) {
	return service.Tenant().PlanList(ctx, req)
}
