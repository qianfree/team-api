package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PricingFetchOfficial(ctx context.Context, req *v1.PricingFetchOfficialReq) (res *v1.PricingFetchOfficialRes, err error) {
	return service.Admin().FetchOfficialPricing(ctx, req)
}
