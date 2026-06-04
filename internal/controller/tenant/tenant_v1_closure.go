package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRequestClosure(ctx context.Context, req *v1.TenantRequestClosureReq) (res *v1.TenantRequestClosureRes, err error) {
	return service.Tenant().RequestClosure(ctx, req)
}
func (c *ControllerV1) TenantCancelClosure(ctx context.Context, req *v1.TenantCancelClosureReq) (res *v1.TenantCancelClosureRes, err error) {
	return service.Tenant().CancelClosure(ctx, req)
}
