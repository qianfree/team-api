package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PromoCodeCreate(ctx context.Context, req *v1.PromoCodeCreateReq) (res *v1.PromoCodeCreateRes, err error) {
	return service.Admin().CreatePromoCode(ctx, req)
}
