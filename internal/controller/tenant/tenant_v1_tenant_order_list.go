package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrderList(ctx context.Context, req *v1.TenantOrderListReq) (res *v1.TenantOrderListRes, err error) {
	return service.Tenant().OrderList(ctx, req)
}
