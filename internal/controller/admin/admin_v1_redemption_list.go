package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) RedemptionList(ctx context.Context, req *v1.RedemptionListReq) (res *v1.RedemptionListRes, err error) {
	return service.Admin().ListRedemptions(ctx, req)
}
