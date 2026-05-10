package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantValidatePromoCode(ctx context.Context, req *v1.TenantValidatePromoCodeReq) (res *v1.TenantValidatePromoCodeRes, err error) {
	return service.Tenant().ValidatePromoCode(ctx, req)
}
