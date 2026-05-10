package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRedeemCode(ctx context.Context, req *v1.TenantRedeemCodeReq) (res *v1.TenantRedeemCodeRes, err error) {
	return service.Tenant().RedeemCode(ctx, req)
}
