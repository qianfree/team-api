package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PromoCodeUpdate(ctx context.Context, req *v1.PromoCodeUpdateReq) (res *v1.PromoCodeUpdateRes, err error) {
	return service.Admin().UpdatePromoCode(ctx, req)
}
