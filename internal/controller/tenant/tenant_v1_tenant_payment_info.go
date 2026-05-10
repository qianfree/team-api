package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPaymentInfo(ctx context.Context, req *v1.TenantPaymentInfoReq) (res *v1.TenantPaymentInfoRes, err error) {
	return service.Tenant().PaymentInfo(ctx, req)
}
