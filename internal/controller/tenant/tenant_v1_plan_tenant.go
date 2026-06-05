package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPlanList(ctx context.Context, req *v1.TenantPlanListReq) (res *v1.TenantPlanListRes, err error) {
	return service.Tenant().PlanList(ctx, req)
}
func (c *ControllerV1) TenantPlanCurrent(ctx context.Context, req *v1.TenantPlanCurrentReq) (res *v1.TenantPlanCurrentRes, err error) {
	return service.Tenant().PlanCurrent(ctx, req)
}
func (c *ControllerV1) TenantPlanCancelAutoRenew(ctx context.Context, req *v1.TenantPlanCancelAutoRenewReq) (res *v1.TenantPlanCancelAutoRenewRes, err error) {
	return service.Tenant().PlanCancelAutoRenew(ctx, req)
}
