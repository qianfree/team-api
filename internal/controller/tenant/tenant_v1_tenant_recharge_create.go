package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRechargeCreate(ctx context.Context, req *v1.TenantRechargeCreateReq) (res *v1.TenantRechargeCreateRes, err error) {
	return service.Tenant().RechargeCreate(ctx, req)
}
