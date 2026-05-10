package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) RedemptionUsages(ctx context.Context, req *v1.RedemptionUsagesReq) (res *v1.RedemptionUsagesRes, err error) {
	return service.Admin().ListRedemptionUsages(ctx, req)
}
