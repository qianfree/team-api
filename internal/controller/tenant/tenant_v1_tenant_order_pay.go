package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrderPay(ctx context.Context, req *v1.TenantOrderPayReq) (res *v1.TenantOrderPayRes, err error) {
	return service.Tenant().OrderPay(ctx, req)
}
