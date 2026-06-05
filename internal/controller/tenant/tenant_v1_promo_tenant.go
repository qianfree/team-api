package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRedeemCode(ctx context.Context, req *v1.TenantRedeemCodeReq) (res *v1.TenantRedeemCodeRes, err error) {
	return service.Tenant().RedeemCode(ctx, req)
}
func (c *ControllerV1) TenantValidatePromoCode(ctx context.Context, req *v1.TenantValidatePromoCodeReq) (res *v1.TenantValidatePromoCodeRes, err error) {
	return service.Tenant().ValidatePromoCode(ctx, req)
}
func (c *ControllerV1) TenantRedemptionUsages(ctx context.Context, req *v1.TenantRedemptionUsagesReq) (res *v1.TenantRedemptionUsagesRes, err error) {
	return service.Tenant().ListRedemptionUsages(ctx, req)
}
