package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PricingGet(ctx context.Context, req *v1.PricingGetReq) (res *v1.PricingGetRes, err error) {
	return service.Admin().GetModelPricing(ctx, req)
}
