package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PricingList(ctx context.Context, req *v1.PricingListReq) (res *v1.PricingListRes, err error) {
	return service.Admin().ListModelPricing(ctx, req)
}
