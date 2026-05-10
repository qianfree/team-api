package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PromoCodeUsages(ctx context.Context, req *v1.PromoCodeUsagesReq) (res *v1.PromoCodeUsagesRes, err error) {
	return service.Admin().GetPromoCodeUsages(ctx, req)
}
