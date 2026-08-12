package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) MarketplaceList(ctx context.Context, req *v1.MarketplaceListReq) (res *v1.MarketplaceListRes, err error) {
	return service.Tenant().GetModelList(ctx, req)
}
func (c *ControllerV1) MarketplaceDetail(ctx context.Context, req *v1.MarketplaceDetailReq) (res *v1.MarketplaceDetailRes, err error) {
	return service.Tenant().GetModelDetail(ctx, req)
}
