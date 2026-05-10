package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrderCancel(ctx context.Context, req *v1.TenantOrderCancelReq) (res *v1.TenantOrderCancelRes, err error) {
	return service.Tenant().OrderCancel(ctx, req)
}
