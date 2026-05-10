package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPlanCancelAutoRenew(ctx context.Context, req *v1.TenantPlanCancelAutoRenewReq) (res *v1.TenantPlanCancelAutoRenewRes, err error) {
	return service.Tenant().PlanCancelAutoRenew(ctx, req)
}
