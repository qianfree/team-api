package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) RedemptionDisable(ctx context.Context, req *v1.RedemptionDisableReq) (res *v1.RedemptionDisableRes, err error) {
	return service.Admin().DisableRedemption(ctx, req)
}
