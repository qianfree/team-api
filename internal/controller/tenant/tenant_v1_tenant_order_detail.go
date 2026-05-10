package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrderDetail(ctx context.Context, req *v1.TenantOrderDetailReq) (res *v1.TenantOrderDetailRes, err error) {
	return service.Tenant().OrderDetail(ctx, req)
}
