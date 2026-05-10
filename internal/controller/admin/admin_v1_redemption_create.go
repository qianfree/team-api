package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) RedemptionCreate(ctx context.Context, req *v1.RedemptionCreateReq) (res *v1.RedemptionCreateRes, err error) {
	return service.Admin().BatchCreateRedemptions(ctx, req)
}
